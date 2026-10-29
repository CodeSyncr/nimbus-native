// A desktop host for the Nimbus Native C ABI. It drives the Go app in app/
// through nn.h exactly as the iOS and Android hosts do — a UI thread that
// waits for wake, drains, and delivers — and checks the contract: error
// codes, panics, ordering, batching, latency, and leaks over 100k+ calls.
//
// Built and run by bridge/abi_test.go. Prints "ok …" lines; exits 1 on the
// first failure.

#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "nn.h"

#ifdef __APPLE__
#include <mach/mach.h>
static long long footprint(void) {
	task_vm_info_data_t info;
	mach_msg_type_number_t n = TASK_VM_INFO_COUNT;
	if (task_info(mach_task_self(), TASK_VM_INFO, (task_info_t)&info, &n) != KERN_SUCCESS) return -1;
	return (long long)info.phys_footprint;
}
#else
static long long footprint(void) {
	long pages = 0;
	FILE *f = fopen("/proc/self/statm", "r");
	if (f) { if (fscanf(f, "%*ld %ld", &pages) != 1) pages = 0; fclose(f); }
	return (long long)pages * 4096;
}
#endif

#define FAIL(...) do { printf("FAIL: " __VA_ARGS__); printf("\n"); exit(1); } while (0)

// ---- wake: Go threads signal, the "UI thread" (main) waits -----------------

static pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t cv = PTHREAD_COND_INITIALIZER;
static int woken;

static void on_wake(void *ctx) {
	if (ctx != (void *)0x5eed) { printf("FAIL: wake ctx not passed through\n"); exit(1); }
	pthread_mutex_lock(&mu);
	woken = 1;
	pthread_cond_signal(&cv);
	pthread_mutex_unlock(&mu);
}

// Waits for a wake (up to 5 s), then drains. Calls fn for each message.
// Returns the number of messages delivered.
typedef void (*msg_fn)(const uint8_t *p, int32_t n, void *arg);
static int pump(msg_fn fn, void *arg) {
	pthread_mutex_lock(&mu);
	struct timespec dl;
	clock_gettime(CLOCK_REALTIME, &dl);
	dl.tv_sec += 5;
	while (!woken) {
		if (pthread_cond_timedwait(&cv, &mu, &dl) != 0) { pthread_mutex_unlock(&mu); FAIL("no wake within 5 s"); }
	}
	woken = 0;
	pthread_mutex_unlock(&mu);

	uint8_t *buf = NULL;
	int32_t len = 0;
	if (nn_drain(&buf, &len) != NN_OK) FAIL("nn_drain");
	int count = 0;
	for (int32_t off = 0; off < len;) {
		if (len - off < 4) FAIL("truncated frame header");
		uint32_t n = (uint32_t)buf[off] | (uint32_t)buf[off + 1] << 8 | (uint32_t)buf[off + 2] << 16 | (uint32_t)buf[off + 3] << 24;
		off += 4;
		if ((int64_t)n > len - off) FAIL("truncated frame body");
		fn(buf + off, (int32_t)n, arg);
		off += (int32_t)n;
		count++;
	}
	nn_free(buf);
	return count;
}

// ---- helpers ------------------------------------------------------------------

static double now_us(void) {
	struct timespec t;
	clock_gettime(CLOCK_MONOTONIC, &t);
	return t.tv_sec * 1e6 + t.tv_nsec / 1e3;
}

static int32_t call(const char *req, char *out, size_t cap) {
	uint8_t *p = NULL;
	int32_t n = 0;
	int32_t rc = nn_call((const uint8_t *)req, (int32_t)strlen(req), &p, &n);
	if (out && cap) {
		size_t k = (size_t)n < cap - 1 ? (size_t)n : cap - 1;
		if (p) memcpy(out, p, k);
		out[k] = 0;
	}
	if (p) nn_free(p);
	return rc;
}

static void send_str(const char *s) {
	if (nn_send((const uint8_t *)s, (int32_t)strlen(s)) != NN_OK) FAIL("nn_send %s", s);
}

typedef struct { long long sent, wakes, drains, received, calls, panics, goroutines, heap; } stats;

static stats read_stats(int gc) {
	char buf[256];
	stats s = {0};
	if (call(gc ? "stats:gc" : "stats:", buf, sizeof buf) != NN_OK) FAIL("stats");
	sscanf(buf, "%lld %lld %lld %lld %lld %lld %lld %lld", &s.sent, &s.wakes, &s.drains, &s.received,
	       &s.calls, &s.panics, &s.goroutines, &s.heap);
	return s;
}

// ---- message sinks ------------------------------------------------------------

typedef struct { int goroutines; int *next; long total; } burst_state;

static void on_burst(const uint8_t *p, int32_t n, void *arg) {
	burst_state *b = arg;
	char tmp[32];
	if (n <= 0 || n >= (int32_t)sizeof tmp) FAIL("odd burst message length %d", n);
	memcpy(tmp, p, (size_t)n);
	tmp[n] = 0;
	int g, i;
	if (sscanf(tmp, "%d:%d", &g, &i) != 2 || g < 0 || g >= b->goroutines) FAIL("bad burst message %s", tmp);
	if (i != b->next[g]) FAIL("goroutine %d: got message %d, want %d (order broken)", g, i, b->next[g]);
	b->next[g]++;
	b->total++;
}

static void on_pong(const uint8_t *p, int32_t n, void *arg) {
	char want[32];
	snprintf(want, sizeof want, "pong:%d", *(int *)arg);
	if ((size_t)n != strlen(want) || memcmp(p, want, (size_t)n) != 0) FAIL("got %.*s, want %s", n, (const char *)p, want);
}

// ---- the checks -----------------------------------------------------------------

static void burst(int goroutines, int each, int *drains_out) {
	int next[64] = {0};
	burst_state b = {goroutines, next, 0};
	char out[16];
	char req[64];
	snprintf(req, sizeof req, "burst:%d:%d", goroutines, each);
	if (call(req, out, sizeof out) != NN_OK) FAIL("burst call");
	int drains = 0;
	while (b.total < (long)goroutines * each) {
		pump(on_burst, &b);
		drains++;
	}
	*drains_out = drains;
}

static void soak(int n) {
	char out[64];
	for (int i = 0; i < n; i++) {
		if (call("echo:soak", out, sizeof out) != NN_OK || strcmp(out, "soak") != 0) FAIL("soak call %d", i);
	}
	for (int i = 0; i < n; i++) {
		char ping[32];
		snprintf(ping, sizeof ping, "ping:%d", i);
		send_str(ping);
		pump(on_pong, &i);
	}
	int drains;
	burst(4, n / 4, &drains);
}

int main(int argc, char **argv) {
	char out[4096];
	if (argc < 2) FAIL("usage: host <schema hash>");
	uint64_t schema = strtoull(argv[1], NULL, 0);

	// Before start: everything refuses politely.
	if (nn_send((const uint8_t *)"x", 1) != NN_ERR_STATE) FAIL("send before start");
	uint8_t *p; int32_t n;
	if (nn_drain(&p, &n) != NN_ERR_STATE) FAIL("drain before start");
	if (call("echo:x", NULL, 0) != NN_ERR_STATE) FAIL("call before start");
	printf("ok   refuses before start\n");

	nn_host bad = {NN_ABI_VERSION + 1, schema, (void *)0x5eed, on_wake};
	if (nn_start(&bad) != NN_ERR_ABI) FAIL("wrong ABI accepted");
	nn_host nowake = {NN_ABI_VERSION, schema, NULL, NULL};
	if (nn_start(&nowake) != NN_ERR_ARG) FAIL("missing wake accepted");
	if (nn_start(NULL) != NN_ERR_ARG) FAIL("NULL host accepted");
	nn_host stale = {NN_ABI_VERSION, schema ^ 1, (void *)0x5eed, on_wake};
	if (nn_start(&stale) != NN_ERR_SCHEMA) FAIL("wrong schema accepted");
	nn_host host = {NN_ABI_VERSION, schema, (void *)0x5eed, on_wake};
	if (nn_start(&host) != NN_OK) FAIL("start");
	if (nn_start(&host) != NN_ERR_STATE) FAIL("second start accepted");
	if (nn_drain(NULL, NULL) != NN_ERR_ARG) FAIL("drain with NULL out accepted");
	printf("ok   start: ABI check, schema check, argument checks, single start\n");

	if (nn_drain(&p, &n) != NN_OK || p != NULL || n != 0) FAIL("empty drain");
	if (call("echo:héllo wörld", out, sizeof out) != NN_OK || strcmp(out, "héllo wörld") != 0) FAIL("echo: %s", out);
	if (call("echo:", out, sizeof out) != NN_OK || out[0] != 0) FAIL("empty echo");
	printf("ok   call round trip (UTF-8, empty)\n");

	if (call("panic", out, sizeof out) != NN_ERR_PANIC) FAIL("panic not reported");
	if (strstr(out, "asked to panic") == NULL || strstr(out, "goroutine") == NULL) FAIL("panic text lacks message or stack");
	if (call("echo:alive", out, sizeof out) != NN_OK || strcmp(out, "alive") != 0) FAIL("runtime dead after panic");
	printf("ok   panic recovered with message and Go stack; runtime still alive\n");

	// Ordering and batching: 8 goroutines × 25k messages each.
	int drains;
	double t0 = now_us();
	burst(8, 25000, &drains);
	double dt = now_us() - t0;
	printf("ok   burst: 200000 messages from 8 goroutines, per-goroutine order kept, %d drains (%.0f msgs/drain), %.0f ns/msg\n",
	       drains, 200000.0 / drains, dt * 1000 / 200000);

	// Latency: sequential ping-pong, one wake + drain per round trip.
	int rounds = 10000;
	t0 = now_us();
	for (int i = 0; i < rounds; i++) {
		char ping[32];
		snprintf(ping, sizeof ping, "ping:%d", i);
		send_str(ping);
		pump(on_pong, &i);
	}
	printf("ok   ping-pong: %d round trips, avg %.2f µs\n", rounds, (now_us() - t0) / rounds);

	t0 = now_us();
	for (int i = 0; i < 100000; i++) call("echo:x", NULL, 0);
	printf("ok   sync call: avg %.3f µs over 100000\n", (now_us() - t0) / 100000);

	// Leaks: soak twice; the second round must not grow memory.
	int soakN = 100000;
	soak(soakN);
	stats s1 = read_stats(1);
	long long f1 = footprint();
	soak(soakN);
	stats s2 = read_stats(1);
	long long f2 = footprint();
	long long heapGrowth = s2.heap - s1.heap, fpGrowth = f2 - f1;
	printf("info soak round: %d calls + %d ping-pongs + %d burst msgs\n", soakN, soakN, soakN);
	printf("info Go heap %lld → %lld bytes (%+lld); footprint %lld → %lld KB (%+lld KB); goroutines %lld → %lld\n",
	       s1.heap, s2.heap, heapGrowth, f1 / 1024, f2 / 1024, fpGrowth / 1024, s1.goroutines, s2.goroutines);
	if (heapGrowth > 512 * 1024) FAIL("Go heap grew %lld bytes over a soak round", heapGrowth);
	if (fpGrowth > 4 * 1024 * 1024) FAIL("process footprint grew %lld KB over a soak round", fpGrowth / 1024);
	if (s2.goroutines > s1.goroutines + 2) FAIL("goroutines grew %lld → %lld", s1.goroutines, s2.goroutines);
	printf("ok   no leak over a 300k-operation soak round\n");

	printf("info totals: sent %lld, wakes %lld, drains %lld, received %lld, calls %lld, panics %lld\n",
	       s2.sent, s2.wakes, s2.drains, s2.received, s2.calls, s2.panics);
	printf("PASS\n");
	return 0;
}
