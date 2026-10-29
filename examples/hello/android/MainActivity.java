package space.nimbusgo.hello;

import android.app.Activity;
import android.graphics.Typeface;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.widget.ScrollView;
import android.widget.TextView;
import java.io.BufferedReader;
import java.io.FileReader;
import space.nimbusgo.nativekit.Bridge;
import space.nimbusgo.nativekit.BridgeException;
import space.nimbusgo.nativekit.WireException;

/**
 * Bridge and wire checks, run at launch. Everything crosses as generated Proto messages. Every line
 * also goes to logcat (tag NimbusNativeApp).
 */
public class MainActivity extends Activity {
    private static final int PING_COUNT = 1000, BURST_COUNT = 100_000, SOAK_CALLS = 100_000;
    private static final int PING = 0, BURST = 1, SOAK1 = 2, SOAK2 = 3, DONE = 4;

    private TextView text;
    private final StringBuilder lines = new StringBuilder();
    private int stage = PING, ticks, pings, burstNext;
    private long t0, drainsBefore;
    private boolean soakOK;
    private long[] soakA; // heap, rss, goroutines after round 1

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        text = new TextView(this);
        text.setTypeface(Typeface.MONOSPACE);
        text.setTextSize(11);
        int pad = (int) (16 * getResources().getDisplayMetrics().density);
        text.setPadding(pad, pad * 3, pad, pad);
        ScrollView scroll = new ScrollView(this);
        scroll.addView(text);
        setContentView(scroll);

        Bridge.setListener(bytes -> {
            try {
                received(Proto.decode(bytes));
            } catch (WireException e) {
                fail("undecodable message from Go: " + e.getMessage());
            }
        });
        try {
            run();
        } catch (RuntimeException e) {
            fail(e.toString());
        }
    }

    private void run() {
        try {
            Bridge.start(NimbusSchema.COMBINED ^ 1);
            fail("a stale schema was accepted");
            return;
        } catch (BridgeException e) {
            if (e.code != Bridge.ERR_SCHEMA) throw e;
            log("schema guard: stale hash refused");
        }

        t0 = SystemClock.elapsedRealtimeNanos();
        Bridge.start(NimbusSchema.COMBINED);
        log(String.format("start: ok in %.2f ms, schema %x", since(t0) / 1e3, NimbusSchema.COMBINED));

        Proto.Hello hello = new Proto.Hello();
        hello.name = "Yash";
        Proto.Greeting g = (Proto.Greeting) call(hello);
        log("call: " + g.text + " " + g.goVersion + " on " + g.platform);

        int n = 10_000;
        Proto.Echo echo = new Proto.Echo();
        echo.text = "x";
        long b0 = SystemClock.elapsedRealtimeNanos();
        for (int i = 0; i < n; i++) call(echo);
        log(String.format("typed sync calls: %d, avg %.2f µs (encode + call + decode)", n, since(b0) / n));

        try {
            Proto.Crash crash = new Proto.Crash();
            crash.reason = "asked to panic";
            call(crash);
            fail("panic not reported");
            return;
        } catch (BridgeException e) {
            boolean stack = e.getMessage().contains("asked to panic") && e.getMessage().contains("goroutine");
            log("panic: recovered (code " + e.code + "), message + Go stack " + (stack ? "delivered" : "MISSING"));
        }

        Proto.SnapshotRequest req = new Proto.SnapshotRequest();
        req.rows = 1000;
        int rows = 0;
        long s0 = SystemClock.elapsedRealtimeNanos();
        for (int i = 0; i < 20; i++) {
            Proto.Snapshot s = (Proto.Snapshot) call(req);
            rows += s.rows.size();
            if (s.rows.get(2).subtitle == null || s.rows.get(1).subtitle != null
                    || !"Pune".equals(s.rows.get(999).attrs.get("city")) || s.rows.get(5).kind != Proto.Kind.BUTTON) {
                fail("snapshot fields wrong");
                return;
            }
        }
        log(String.format("snapshot: %d rows decoded, %.2f µs/row (Go encode + copy + Java decode)", rows, since(s0) / rows));

        t0 = SystemClock.elapsedRealtimeNanos();
        send(ping(0));
    }

    private void received(Proto.Message m) {
        if (m instanceof Proto.Ready) {
            log("go → native: ready (" + ((Proto.Ready) m).goVersion + ")");
        } else if (m instanceof Proto.Tick) {
            ticks++;
            log("go → native: tick " + ((Proto.Tick) m).n + " (goroutine)");
            maybePass();
        } else if (stage == PING && m instanceof Proto.Pong) {
            long seq = ((Proto.Pong) m).seq;
            if (seq != pings) {
                fail("pong " + seq + ", want " + pings);
                return;
            }
            if (++pings < PING_COUNT) {
                send(ping(pings));
            } else {
                log(String.format("ping-pong: %d typed round trips, avg %.2f µs", PING_COUNT, since(t0) / PING_COUNT));
                startBurst(BURST);
            }
        } else if ((stage == BURST || stage == SOAK1 || stage == SOAK2) && m instanceof Proto.Item) {
            long index = ((Proto.Item) m).index;
            if (index != burstNext) {
                fail("burst out of order: " + index + ", want " + burstNext);
                return;
            }
            if (++burstNext == BURST_COUNT) burstDone();
        } else if (stage != DONE) {
            fail("unexpected message " + m.getClass().getSimpleName());
        }
    }

    private void startBurst(int next) {
        stage = next;
        burstNext = 0;
        drainsBefore = stats(false).drains;
        t0 = SystemClock.elapsedRealtimeNanos();
        Proto.StartBurst b = new Proto.StartBurst();
        b.count = BURST_COUNT;
        call(b);
    }

    private void burstDone() {
        long drains = stats(false).drains - drainsBefore;
        if (stage == BURST) {
            log(String.format("burst: %d typed msgs in order, %d drains, %.0f ns/msg", BURST_COUNT, drains, since(t0) * 1000 / BURST_COUNT));
            soakRound(SOAK1);
            return;
        }
        Proto.Stats s = stats(true);
        long[] mem = {s.heap, rss(), s.goroutines};
        if (stage == SOAK1) {
            soakA = mem;
            soakRound(SOAK2);
            return;
        }
        log(String.format("soak: 2 rounds × (%d calls + %d msgs)", SOAK_CALLS, BURST_COUNT));
        log(String.format("  Go heap %d → %d KB, RSS %d → %d MB, goroutines %d → %d",
                soakA[0] / 1024, mem[0] / 1024, soakA[1] >> 20, mem[1] >> 20, soakA[2], mem[2]));
        if (mem[0] - soakA[0] > 512 * 1024 || mem[1] - soakA[1] > 4 << 20 || mem[2] > soakA[2] + 2) {
            fail("memory grew between soak rounds");
            return;
        }
        log("soak: no leak");
        soakOK = true;
        stage = DONE;
        maybePass();
    }

    private void soakRound(int next) {
        Proto.Echo echo = new Proto.Echo();
        echo.text = "soak";
        for (int i = 0; i < SOAK_CALLS; i++) call(echo);
        startBurst(next);
    }

    private void maybePass() {
        if (soakOK && ticks == 3) log("NN-0C RESULT: PASS");
    }

    // ---- helpers ----

    private static Proto.Message call(Proto.Message m) {
        return Proto.decode(Bridge.call(Proto.encode(m)));
    }

    private static void send(Proto.Message m) {
        Bridge.send(Proto.encode(m));
    }

    private static Proto.Ping ping(int seq) {
        Proto.Ping p = new Proto.Ping();
        p.seq = seq;
        return p;
    }

    private static Proto.Stats stats(boolean gc) {
        Proto.StatsRequest r = new Proto.StatsRequest();
        r.gc = gc;
        return (Proto.Stats) call(r);
    }

    /** Resident set size in bytes, from /proc/self/statm. */
    private static long rss() {
        try (BufferedReader r = new BufferedReader(new FileReader("/proc/self/statm"))) {
            return Long.parseLong(r.readLine().split(" ")[1]) * 4096;
        } catch (Exception e) {
            return 0;
        }
    }

    private static double since(long t) {
        return (SystemClock.elapsedRealtimeNanos() - t) / 1e3;
    }

    private void fail(String s) {
        stage = DONE;
        log("FAIL: " + s);
    }

    private void log(String s) {
        Log.i("NimbusNativeApp", "[hello] " + s);
        lines.append(s).append('\n');
        text.setText("Nimbus Native · bridge + wire checks\n\n" + lines);
    }
}
