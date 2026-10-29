package space.nimbusgo.hello;

import android.app.Activity;
import android.graphics.Typeface;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.widget.ScrollView;
import android.widget.TextView;
import java.nio.charset.StandardCharsets;
import space.nimbusgo.nativekit.Bridge;

/** Phase 0a checks, run at launch. Every line also goes to logcat (tag NimbusNativeApp). */
public class MainActivity extends Activity {
    private static final int PING_COUNT = 1000;

    private TextView text;
    private final StringBuilder lines = new StringBuilder();
    private int pings, ticks;
    private long pingStart;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        text = new TextView(this);
        text.setTypeface(Typeface.MONOSPACE);
        text.setTextSize(13);
        int pad = (int) (16 * getResources().getDisplayMetrics().density);
        text.setPadding(pad, pad * 3, pad, pad);
        ScrollView scroll = new ScrollView(this);
        scroll.addView(text);
        setContentView(scroll);
        run();
    }

    private void run() {
        long t0 = SystemClock.elapsedRealtimeNanos();
        Bridge.setListener(msg -> received(new String(msg, StandardCharsets.UTF_8)));
        try {
            Bridge.start();
            log(String.format("start: ok in %.1f ms", (SystemClock.elapsedRealtimeNanos() - t0) / 1e6));

            log("call: " + new String(Bridge.call(bytes("Yash")), StandardCharsets.UTF_8));

            int n = 10_000;
            long b0 = SystemClock.elapsedRealtimeNanos();
            for (int i = 0; i < n; i++) Bridge.call(bytes("bench"));
            log(String.format("sync calls: %d, avg %.2f µs", n, (SystemClock.elapsedRealtimeNanos() - b0) / 1e3 / n));

            try {
                Bridge.call(bytes("panic"));
                log("panic: FAIL (no error)");
            } catch (IllegalStateException e) {
                log("panic: recovered (" + e.getMessage() + ")");
            }

            pingStart = SystemClock.elapsedRealtimeNanos();
            Bridge.send(bytes("ping:0"));
        } catch (RuntimeException e) {
            log("FAIL: " + e);
        }
    }

    private void received(String msg) {
        if (msg.equals("ready")) {
            log("go → native: ready");
        } else if (msg.startsWith("tick:")) {
            ticks++;
            log("go → native: " + msg + " (goroutine)");
        } else if (msg.startsWith("pong:")) {
            pings++;
            if (pings < PING_COUNT) {
                Bridge.send(bytes("ping:" + pings));
            } else {
                log(String.format("async ping-pong: %d, avg %.2f µs round trip",
                        PING_COUNT, (SystemClock.elapsedRealtimeNanos() - pingStart) / 1e3 / PING_COUNT));
            }
        }
        if (ticks == 3 && pings == PING_COUNT) {
            ticks++; // report once
            log("NN-0A RESULT: PASS");
        }
    }

    private void log(String s) {
        Log.i("NimbusNativeApp", "[hello] " + s);
        lines.append(s).append('\n');
        text.setText("Nimbus Native · Phase 0a\n\n" + lines);
    }

    private static byte[] bytes(String s) {
        return s.getBytes(StandardCharsets.UTF_8);
    }
}
