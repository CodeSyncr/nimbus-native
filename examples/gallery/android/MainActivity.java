package space.nimbusgo.gallery;

import android.app.Activity;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.SystemClock;
import android.util.Log;
import android.view.Choreographer;
import android.view.View;
import android.widget.TextView;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import space.nimbusgo.nativekit.NimbusView;
import space.nimbusgo.nativekit.Render;
import space.nimbusgo.nativekit.Renderer;

/**
 * The gallery's Android host: one NimbusView, plus a benchmark that runs by itself after launch
 * and logs to logcat (tag NimbusNativeApp).
 */
public class MainActivity extends Activity {
    private NimbusView nimbus;
    private Renderer r;
    private final Handler main = new Handler(Looper.getMainLooper());
    private int commits;
    private long pressAt;
    private boolean typing;
    private final List<Double> taps = new ArrayList<>();
    private final List<Long> frameTimes = new ArrayList<>();

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        nimbus = new NimbusView(this, NimbusSchema.COMBINED);
        setContentView(nimbus);
        r = nimbus.renderer;
        r.onCommit = this::committed;
    }

    private void committed(Render.Commit c, double applyMs) {
        commits++;
        if (typing) {
            typing = false;
            View g = r.viewForTag(r.tagForTestID("greeting"));
            String greeting = g instanceof TextView ? ((TextView) g).getText().toString() : "?";
            log("typed \"Yash\" → greeting: " + greeting);
            if (!greeting.equals("Hello, Yash 👋")) {
                log("FAIL: greeting did not follow the input");
                return;
            }
            main.post(this::scroll);
            return;
        }
        if (commits == 1) {
            int views = 0;
            for (Render.Op op : c.ops) if (op instanceof Render.Create) views++;
            log(String.format("first render: %d native views, %d ops; Go build %.1f ms, layout %.1f ms (%d text measurements); apply %.1f ms",
                    views, c.ops.size(), c.buildNs / 1e6, c.layoutNs / 1e6, c.measures, applyMs));
            main.postDelayed(this::tap, 1500);
            return;
        }
        if (pressAt == 0) return;
        // Count to the end of the frame that shows the change, not just the apply.
        long pressed = pressAt;
        pressAt = 0;
        double apply = applyMs;
        Choreographer.getInstance().postFrameCallback(t -> main.post(() -> {
            taps.add((SystemClock.elapsedRealtimeNanos() - pressed) / 1e6);
            if (taps.size() == 1) {
                log(String.format("tap commit: %d ops; Go build %.2f ms, layout %.2f ms; apply %.2f ms",
                        c.ops.size(), c.buildNs / 1e6, c.layoutNs / 1e6, apply));
            }
            if (taps.size() < 20) {
                main.postDelayed(this::tap, 50);
                return;
            }
            List<Double> s = new ArrayList<>(taps);
            Collections.sort(s);
            log(String.format("tap → screen: 20 taps, median %.2f ms, worst %.2f ms", s.get(10), s.get(19)));
            View counter = r.viewForTag(r.tagForTestID("counter"));
            String label = counter instanceof android.view.ViewGroup && ((android.view.ViewGroup) counter).getChildAt(0) instanceof TextView
                    ? ((TextView) ((android.view.ViewGroup) counter).getChildAt(0)).getText().toString() : "?";
            log("counter label now: " + label);
            typeName();
        }));
    }

    private void tap() {
        int tag = r.tagForTestID("counter");
        if (tag == 0) {
            log("FAIL: no counter");
            return;
        }
        pressAt = SystemClock.elapsedRealtimeNanos();
        r.press(tag);
    }

    /** Types into the name field the way the keyboard does. */
    private void typeName() {
        View f = r.viewForTag(r.tagForTestID("name"));
        if (!(f instanceof android.widget.EditText)) {
            log("FAIL: no name field");
            return;
        }
        typing = true;
        ((android.widget.EditText) f).setText("Yash");
    }

    private void scroll() {
        View list = r.viewForTag(r.tagForTestID("list"));
        if (list == null) {
            log("FAIL: no list");
            return;
        }
        int step = Math.round(40 * getResources().getDisplayMetrics().density);
        Choreographer.getInstance().postFrameCallback(new Choreographer.FrameCallback() {
            @Override
            public void doFrame(long t) {
                frameTimes.add(t);
                list.scrollBy(0, step);
                if (frameTimes.size() < 180) {
                    Choreographer.getInstance().postFrameCallback(this);
                    return;
                }
                float hz = getWindowManager().getDefaultDisplay().getRefreshRate();
                double target = 1000.0 / hz;
                int dropped = 0;
                for (int i = 1; i < frameTimes.size(); i++) {
                    if ((frameTimes.get(i) - frameTimes.get(i - 1)) / 1e6 > target * 1.5) dropped++;
                }
                double secs = (frameTimes.get(frameTimes.size() - 1) - frameTimes.get(0)) / 1e9;
                log(String.format("scroll: 180 frames through the list, %.1f fps, %d dropped (target %.1f ms/frame), now at y=%d",
                        (frameTimes.size() - 1) / secs, dropped, target, list.getScrollY()));
                List<Double> s = new ArrayList<>(taps);
                Collections.sort(s);
                log(s.get(10) < 16 && dropped < 9 ? "NN-0D RESULT: PASS" : "NN-0D RESULT: CHECK (see numbers above)");
            }
        });
    }

    private static void log(String s) {
        Log.i("NimbusNativeApp", "[gallery] " + s);
    }
}
