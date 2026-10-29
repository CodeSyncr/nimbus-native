package space.nimbusgo.nativekit;

import android.content.Context;
import android.view.View;
import android.widget.HorizontalScrollView;
import android.widget.ScrollView;

/**
 * A scroll view whose children go in {@link #content}, sized to the content size the layout
 * reports. The platform scroller (vertical or horizontal) is made on the first Scroll op.
 */
final class NNScroll extends NNGroup {
    final NNGroup content;
    private View scroller;
    private boolean horizontal;

    NNScroll(Context c, Renderer r) {
        super(c, r);
        content = new NNGroup(c, r);
        // Clipping lets the renderer skip rows outside the visible area; the
        // content is sized to hold all its children, so nothing is cut off.
        content.setClipChildren(true);
        setClipChildren(true); // content must not draw outside the scroll view
    }

    void setContent(int wPx, int hPx, boolean horizontal) {
        if (scroller == null || horizontal != this.horizontal) {
            if (scroller != null) {
                ((android.view.ViewGroup) scroller).removeView(content);
                removeView(scroller);
            }
            this.horizontal = horizontal;
            android.widget.FrameLayout s = horizontal ? new HorizontalScrollView(getContext()) : new ScrollView(getContext());
            s.addView(content);
            scroller = s;
            addView(s);
        }
        content.fixedW = wPx;
        content.fixedH = hPx;
        content.requestLayout();
    }

    /** The platform scroller, once made (for tests and demos). */
    public View scroller() {
        return scroller;
    }

    @Override
    protected void onMeasure(int wSpec, int hSpec) {
        int w = MeasureSpec.getSize(wSpec), h = MeasureSpec.getSize(hSpec);
        if (scroller != null) {
            scroller.measure(MeasureSpec.makeMeasureSpec(w, MeasureSpec.EXACTLY), MeasureSpec.makeMeasureSpec(h, MeasureSpec.EXACTLY));
        }
        setMeasuredDimension(w, h);
    }

    @Override
    protected void onLayout(boolean changed, int l, int t, int r, int b) {
        if (scroller != null) scroller.layout(0, 0, r - l, b - t);
    }
}
