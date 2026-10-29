package space.nimbusgo.baseline;

import android.app.Activity;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.os.Bundle;
import android.util.Log;
import android.util.TypedValue;
import android.view.Choreographer;
import android.view.Gravity;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import java.util.ArrayList;
import java.util.List;

/** The gallery's list as ordinary Android views, with the same scroll benchmark. */
public class MainActivity extends Activity {
    private float d;
    private final List<Long> frameTimes = new ArrayList<>();

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        d = getResources().getDisplayMetrics().density;
        String[] names = {"Asha Rao", "Vikram Shah", "Meera Iyer", "Rohan Das", "Priya Nair", "Kabir Mehta"};
        String[] statuses = {"Paid", "Packed", "Shipped", "Delivered"};
        int[] badges = {0xffdbeafe, 0xfffef3c7, 0xffede9fe, 0xffdcfce7};
        int[] palette = {0xff2563eb, 0xffdb2777, 0xff059669, 0xffd97706, 0xff7c3aed};

        LinearLayout list = new LinearLayout(this);
        list.setOrientation(LinearLayout.VERTICAL);
        for (int i = 0; i < 1000; i++) {
            int id = 10000 + i;
            LinearLayout row = new LinearLayout(this);
            row.setOrientation(LinearLayout.HORIZONTAL);
            row.setGravity(Gravity.CENTER_VERTICAL);
            row.setPadding(dp(16), dp(10), dp(16), dp(10));
            row.setBackground(rounded(0xffffffff, 0));

            FrameLayout avatar = new FrameLayout(this);
            avatar.setBackground(rounded(palette[id % 5], 20));
            TextView initial = text(names[i % 6].substring(0, 1), 16, 0xffffffff, true);
            initial.setGravity(Gravity.CENTER);
            avatar.addView(initial, new FrameLayout.LayoutParams(-1, -1));
            row.addView(avatar, new LinearLayout.LayoutParams(dp(40), dp(40)));

            LinearLayout texts = new LinearLayout(this);
            texts.setOrientation(LinearLayout.VERTICAL);
            TextView title = text("Order #" + id + " · " + names[i % 6], 15, 0xff111827, true);
            title.setMaxLines(1);
            texts.addView(title);
            texts.addView(text((1 + i % 5) + " items · ₹" + (199 + (i * 37) % 2400) + ".50", 13, 0xff6b7280, false));
            LinearLayout.LayoutParams tp = new LinearLayout.LayoutParams(0, -2, 1);
            tp.setMargins(dp(12), 0, dp(12), 0);
            row.addView(texts, tp);

            TextView badge = text(statuses[i % 4], 12, 0xff111827, true);
            badge.setPadding(dp(8), dp(4), dp(8), dp(4));
            badge.setBackground(rounded(badges[i % 4], 8));
            row.addView(badge);

            LinearLayout.LayoutParams rp = new LinearLayout.LayoutParams(-1, -2);
            rp.bottomMargin = dp(1);
            list.addView(row, rp);
        }
        ScrollView scroll = new ScrollView(this);
        scroll.addView(list);
        setContentView(scroll);
        scroll.postDelayed(() -> bench(scroll), 2500);
    }

    private void bench(ScrollView list) {
        int step = dp(40);
        Choreographer.getInstance().postFrameCallback(new Choreographer.FrameCallback() {
            @Override
            public void doFrame(long t) {
                frameTimes.add(t);
                list.scrollBy(0, step);
                if (frameTimes.size() < 180) {
                    Choreographer.getInstance().postFrameCallback(this);
                    return;
                }
                double target = 1000.0 / getWindowManager().getDefaultDisplay().getRefreshRate();
                int dropped = 0;
                for (int i = 1; i < frameTimes.size(); i++) {
                    if ((frameTimes.get(i) - frameTimes.get(i - 1)) / 1e6 > target * 1.5) dropped++;
                }
                double secs = (frameTimes.get(frameTimes.size() - 1) - frameTimes.get(0)) / 1e9;
                Log.i("NimbusNativeApp", String.format("[baseline] scroll: 180 frames, %.1f fps, %d dropped", (frameTimes.size() - 1) / secs, dropped));
            }
        });
    }

    private TextView text(String s, float size, int color, boolean bold) {
        TextView t = new TextView(this);
        t.setText(s);
        t.setTextSize(TypedValue.COMPLEX_UNIT_DIP, size);
        t.setTextColor(color);
        if (bold) t.setTypeface(Typeface.DEFAULT_BOLD);
        return t;
    }

    private GradientDrawable rounded(int color, float radius) {
        GradientDrawable g = new GradientDrawable();
        g.setColor(color);
        g.setCornerRadius(radius * d);
        return g;
    }

    private int dp(float v) {
        return Math.round(v * d);
    }
}
