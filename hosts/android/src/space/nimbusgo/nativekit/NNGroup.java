package space.nimbusgo.nativekit;

import android.content.Context;
import android.view.View;
import android.view.ViewGroup;

/**
 * A container that places each child at the frame the Go layout computed (in px, kept by the
 * renderer). Children may draw outside it unless the box style clips.
 */
class NNGroup extends ViewGroup {
    final Renderer r;
    /** Fixed size for a scroll view's content, which its scroller measures without limits; -1 if unset. */
    int fixedW = -1, fixedH = -1;

    NNGroup(Context c, Renderer r) {
        super(c);
        this.r = r;
        setClipChildren(false);
        setClipToPadding(false);
    }

    @Override
    protected void onMeasure(int wSpec, int hSpec) {
        for (int i = 0; i < getChildCount(); i++) {
            View child = getChildAt(i);
            int[] f = r.frameOf(child);
            child.measure(MeasureSpec.makeMeasureSpec(f[2], MeasureSpec.EXACTLY),
                    MeasureSpec.makeMeasureSpec(f[3], MeasureSpec.EXACTLY));
        }
        int w = fixedW >= 0 ? fixedW : MeasureSpec.getSize(wSpec);
        int h = fixedH >= 0 ? fixedH : MeasureSpec.getSize(hSpec);
        setMeasuredDimension(w, h);
    }

    @Override
    protected void onLayout(boolean changed, int l, int t, int right, int bottom) {
        for (int i = 0; i < getChildCount(); i++) {
            View child = getChildAt(i);
            int[] f = r.frameOf(child);
            child.layout(f[0], f[1], f[0] + f[2], f[1] + f[3]);
        }
    }

    @Override
    public boolean shouldDelayChildPressedState() {
        return false;
    }
}
