package space.nimbusgo.nativekit;

import android.content.Context;
import android.content.res.Configuration;
import android.view.View;
import android.view.ViewGroup;

/**
 * The root of a Nimbus Native app's UI: an Activity's content view. It starts the Go runtime and
 * shows what the app renders.
 *
 * <pre>setContentView(new NimbusView(this, NimbusSchema.COMBINED));</pre>
 */
public final class NimbusView extends ViewGroup {
    public final Renderer renderer;
    private View rootView;
    private int insetTop, insetBottom, insetLeft, insetRight;

    public NimbusView(Context c, long schema) {
        super(c);
        setBackgroundColor(0xfff9fafb);
        renderer = new Renderer(this);
        setOnApplyWindowInsetsListener((v, insets) -> {
            insetTop = insets.getSystemWindowInsetTop();
            insetBottom = insets.getSystemWindowInsetBottom();
            insetLeft = insets.getSystemWindowInsetLeft();
            insetRight = insets.getSystemWindowInsetRight();
            sendViewport();
            return insets;
        });
        renderer.start(schema);
    }

    void setRoot(View v) {
        if (rootView == v) return;
        if (rootView != null) removeView(rootView);
        rootView = v;
        if (v != null) {
            if (v.getParent() != null) ((ViewGroup) v.getParent()).removeView(v);
            addView(v);
        }
    }

    private void sendViewport() {
        boolean dark = (getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
        renderer.sendViewport(getWidth(), getHeight(), insetTop, insetBottom, insetLeft, insetRight, dark);
    }

    @Override
    protected void onAttachedToWindow() {
        super.onAttachedToWindow();
        requestApplyInsets();
    }

    @Override
    protected void onSizeChanged(int w, int h, int oldw, int oldh) {
        super.onSizeChanged(w, h, oldw, oldh);
        post(this::sendViewport);
    }

    @Override
    protected void onMeasure(int wSpec, int hSpec) {
        int w = MeasureSpec.getSize(wSpec), h = MeasureSpec.getSize(hSpec);
        if (rootView != null) {
            int[] f = renderer.frameOf(rootView);
            rootView.measure(MeasureSpec.makeMeasureSpec(f[2], MeasureSpec.EXACTLY), MeasureSpec.makeMeasureSpec(f[3], MeasureSpec.EXACTLY));
        }
        setMeasuredDimension(w, h);
    }

    @Override
    protected void onLayout(boolean changed, int l, int t, int r, int b) {
        if (rootView != null) {
            int[] f = renderer.frameOf(rootView);
            rootView.layout(f[0], f[1], f[0] + f[2], f[1] + f[3]);
        }
    }
}
