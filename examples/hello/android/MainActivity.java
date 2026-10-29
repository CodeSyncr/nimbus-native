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
import java.nio.charset.StandardCharsets;
import space.nimbusgo.nativekit.Bridge;
import space.nimbusgo.nativekit.BridgeException;

/** Bridge checks, run at launch. Every line also goes to logcat (tag NimbusNativeApp). */
public class MainActivity extends Activity {
    private static final int PING_COUNT = 1000, BURST_COUNT = 100_000, SOAK_CALLS = 100_000;
    private static final int PING = 0, BURST = 1, SOAK1 = 2, SOAK2 = 3, DONE = 4;

    private TextView text;
    private final StringBuilder lines = new StringBuilder();
    private int stage = PING, ticks, pings, burstNext, drainsBefore;
    private long t0;
    private boolean soakOK;
    private long[] soakA; // heap, rss, goroutines after round 1

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        text = new TextView(this);
        text.setTypeface(Typeface.MONOSPACE);
        text.setTextSize(12);
        int pad = (int) (16 * getResources().getDisplayMetrics().density);
        text.setPadding(pad, pad * 3, pad, pad);
        ScrollView scroll = new ScrollView(this);
        scroll.addView(text);
        setContentView(scroll);

        Bridge.setListener(msg -> received(new String(msg, StandardCharsets.UTF_8)));
        try {
            run();
        } catch (RuntimeException e) {
            log("FAIL: " + e);
        }
    }

    private void run() {
        t0 = SystemClock.elapsedRealtimeNanos();
        Bridge.start();
        log(String.format("start: ok in %.2f ms", (SystemClock.elapsedRealtimeNanos() - t0) / 1e6));
        log("call: " + text("hello:Yash"));

        int n = 10_000;
        long b0 = SystemClock.elapsedRealtimeNanos();
        for (int i = 0; i < n; i++) Bridge.call(bytes("echo:x"));
        log(String.format("sync calls: %d, avg %.2f µs", n, (SystemClock.elapsedRealtimeNanos() - b0) / 1e3 / n));

        try {
            Bridge.call(bytes("panic"));
            log("FAIL: panic not reported");
        } catch (BridgeException e) {
            boolean stack = e.getMessage().contains("asked to panic") && e.getMessage().contains("goroutine");
            log("panic: recovered (code " + e.code + "), message + Go stack " + (stack ? "delivered" : "MISSING"));
        }

        t0 = SystemClock.elapsedRealtimeNanos();
        Bridge.send(bytes("ping:0"));
    }

    private void received(String msg) {
        if (msg.equals("ready")) {
            log("go → native: ready");
            return;
        }
        if (msg.startsWith("tick:")) {
            ticks++;
            log("go → native: " + msg + " (goroutine)");
            maybePass();
            return;
        }
        if (stage == PING && msg.startsWith("pong:")) {
            pings++;
            if (pings < PING_COUNT) {
                Bridge.send(bytes("ping:" + pings));
            } else {
                log(String.format("ping-pong: %d round trips, avg %.2f µs", PING_COUNT, since(t0) / PING_COUNT));
                startBurst(BURST);
            }
        } else if (stage == BURST || stage == SOAK1 || stage == SOAK2) {
            if (!msg.equals("b:" + burstNext)) {
                fail("burst out of order: got " + msg + ", want b:" + burstNext);
                return;
            }
            if (++burstNext == BURST_COUNT) burstDone();
        }
    }

    private void startBurst(int next) {
        stage = next;
        burstNext = 0;
        drainsBefore = (int) stats(false)[4];
        t0 = SystemClock.elapsedRealtimeNanos();
        Bridge.call(bytes("burst:" + BURST_COUNT));
    }

    private void burstDone() {
        long drains = stats(false)[4] - drainsBefore;
        if (stage == BURST) {
            log(String.format("burst: %d msgs in order, %d drains (%d msgs/drain), %.0f ns/msg",
                    BURST_COUNT, drains, BURST_COUNT / Math.max(drains, 1), since(t0) * 1000 / BURST_COUNT));
            soakRound(SOAK1);
            return;
        }
        long[] s = stats(true);
        long[] mem = {s[0], rss(), s[1]};
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
        for (int i = 0; i < SOAK_CALLS; i++) Bridge.call(bytes("echo:soak"));
        startBurst(next);
    }

    private void maybePass() {
        if (soakOK && ticks == 3) log("NN-0B RESULT: PASS");
    }

    // ---- helpers ----

    /** heap, goroutines, sent, wakes, drains */
    private static long[] stats(boolean gc) {
        String[] f = text(gc ? "stats:gc" : "stats:").split(" ");
        long[] out = new long[f.length];
        for (int i = 0; i < f.length; i++) out[i] = Long.parseLong(f[i]);
        return out;
    }

    /** Resident set size in bytes, from /proc/self/statm. */
    private static long rss() {
        try (BufferedReader r = new BufferedReader(new FileReader("/proc/self/statm"))) {
            return Long.parseLong(r.readLine().split(" ")[1]) * 4096;
        } catch (Exception e) {
            return 0;
        }
    }

    private static String text(String req) {
        return new String(Bridge.call(bytes(req)), StandardCharsets.UTF_8);
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
        text.setText("Nimbus Native · bridge checks\n\n" + lines);
    }

    private static byte[] bytes(String s) {
        return s.getBytes(StandardCharsets.UTF_8);
    }
}
