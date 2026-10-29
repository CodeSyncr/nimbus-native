package space.nimbusgo.nativekit;

import android.content.Context;
import android.graphics.Typeface;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.TextView;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * A navigation stack: a Material-style top bar with a back arrow and the top screen's title, and
 * the screens below it, sliding in and out. Screens' content is Go-rendered; going back by the
 * arrow or the system back button tells Go.
 */
final class NNStack extends ViewGroup {
    private static final int BAR_DP = 56;
    private static final long SLIDE_MS = 250;

    private final Renderer r;
    private final int tag;
    private final float d;
    private final ViewGroup bar;
    private final TextView title, back;
    private final List<View> screens = new ArrayList<>();
    private final Map<View, String> titles = new HashMap<>();
    private final Set<View> leaving = new HashSet<>();
    private int reportedW, reportedH;

    NNStack(Context c, Renderer r, int tag) {
        super(c);
        this.r = r;
        this.tag = tag;
        this.d = c.getResources().getDisplayMetrics().density;
        bar = new android.widget.FrameLayout(c);
        bar.setBackgroundColor(0xffffffff);
        bar.setElevation(4 * d);
        back = new TextView(c);
        back.setText("←");
        back.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 24);
        back.setTextColor(0xff111827);
        back.setGravity(Gravity.CENTER);
        back.setContentDescription("Back");
        back.setOnClickListener(v -> goBack());
        title = new TextView(c);
        title.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 20);
        title.setTypeface(Typeface.create(Typeface.DEFAULT, Typeface.BOLD));
        title.setTextColor(0xff111827);
        title.setGravity(Gravity.CENTER_VERTICAL);
        title.setSingleLine(true);
        bar.addView(back, new android.widget.FrameLayout.LayoutParams(px(56), px(56)));
        bar.addView(title, new android.widget.FrameLayout.LayoutParams(-1, px(56)));
        addView(bar);
    }

    /** Shows exactly these screens, animating a push or pop of one. */
    void setScreens(List<View> wanted) {
        int n = screens.size();
        if (wanted.size() == n + 1 && wanted.subList(0, n).equals(screens)) {
            View in = wanted.get(n);
            screens.add(in);
            attach(in);
            if (n > 0 && getWidth() > 0) slide(screens.get(n - 1), in, true);
        } else if (wanted.size() == n - 1 && screens.subList(0, n - 1).equals(wanted)) {
            View out = screens.remove(n - 1);
            if (getWidth() > 0) slide(screens.get(n - 2), out, false);
            else detach(out);
        } else if (!wanted.equals(screens)) {
            for (View v : screens) if (!wanted.contains(v)) detach(v);
            screens.clear();
            screens.addAll(wanted);
            for (View v : wanted) attach(v);
            for (int i = 0; i < screens.size(); i++) screens.get(i).setVisibility(i == screens.size() - 1 ? VISIBLE : INVISIBLE);
        }
        updateBar();
    }

    void setTitle(View screen, String t) {
        titles.put(screen, t);
        updateBar();
    }

    /** True if the view is a screen sliding out: it must stay until the animation ends. */
    boolean isLeaving(View v) {
        return leaving.contains(v);
    }

    /** The back arrow or system back: pops natively, then tells Go. */
    boolean goBack() {
        if (screens.size() < 2) return false;
        List<View> rest = new ArrayList<>(screens.subList(0, screens.size() - 1));
        setScreens(rest);
        r.popped(tag, rest.size());
        return true;
    }

    int depth() {
        return screens.size();
    }

    private void slide(View under, View top, boolean push) {
        int w = getWidth();
        under.setVisibility(VISIBLE);
        if (push) {
            top.setTranslationX(w);
            top.animate().translationX(0).setDuration(SLIDE_MS).start();
            under.animate().translationX(-w * 0.3f).setDuration(SLIDE_MS)
                    .withEndAction(() -> { if (screens.indexOf(under) != screens.size() - 1) under.setVisibility(INVISIBLE); }).start();
        } else {
            leaving.add(top);
            under.setTranslationX(-w * 0.3f);
            under.animate().translationX(0).setDuration(SLIDE_MS).start();
            top.animate().translationX(w).setDuration(SLIDE_MS).withEndAction(() -> detach(top)).start();
        }
    }

    private void attach(View v) {
        if (v.getParent() == this) return;
        if (v.getParent() != null) ((ViewGroup) v.getParent()).removeView(v);
        addView(v, indexOfChild(bar)); // screens under the bar
        v.setTranslationX(0);
    }

    private void detach(View v) {
        leaving.remove(v);
        titles.remove(v);
        if (v.getParent() == this) removeView(v);
    }

    private void updateBar() {
        View top = screens.isEmpty() ? null : screens.get(screens.size() - 1);
        title.setText(top != null && titles.containsKey(top) ? titles.get(top) : "");
        boolean canGoBack = screens.size() > 1;
        back.setVisibility(canGoBack ? VISIBLE : GONE);
        ((android.widget.FrameLayout.LayoutParams) title.getLayoutParams()).leftMargin = px(canGoBack ? 72 : 16);
        title.requestLayout();
    }

    @Override
    protected void onMeasure(int wSpec, int hSpec) {
        int w = MeasureSpec.getSize(wSpec), h = MeasureSpec.getSize(hSpec), barH = px(BAR_DP);
        bar.measure(MeasureSpec.makeMeasureSpec(w, MeasureSpec.EXACTLY), MeasureSpec.makeMeasureSpec(barH, MeasureSpec.EXACTLY));
        int contentH = Math.max(0, h - barH);
        for (int i = 0; i < getChildCount(); i++) {
            View c = getChildAt(i);
            if (c != bar) c.measure(MeasureSpec.makeMeasureSpec(w, MeasureSpec.EXACTLY), MeasureSpec.makeMeasureSpec(contentH, MeasureSpec.EXACTLY));
        }
        setMeasuredDimension(w, h);
        if (w > 0 && (w != reportedW || contentH != reportedH)) {
            reportedW = w;
            reportedH = contentH;
            post(() -> r.resized(tag, reportedW, reportedH));
        }
    }

    @Override
    protected void onLayout(boolean changed, int l, int t, int right, int bottom) {
        int w = right - l, barH = px(BAR_DP);
        bar.layout(0, 0, w, barH);
        for (int i = 0; i < getChildCount(); i++) {
            View c = getChildAt(i);
            if (c != bar) c.layout(0, barH, w, barH + c.getMeasuredHeight());
        }
    }

    private int px(float dp) {
        return Math.round(dp * d);
    }
}
