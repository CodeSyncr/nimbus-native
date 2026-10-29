package space.nimbusgo.nativekit;

import android.content.Context;

/** A container that reports taps and dims while pressed. */
final class NNPressable extends NNGroup {
    float baseAlpha = 1;

    NNPressable(Context c, Renderer r, int tag) {
        super(c, r);
        setClickable(true);
        setOnClickListener(v -> r.press(tag));
    }

    @Override
    public void setPressed(boolean pressed) {
        super.setPressed(pressed);
        setAlpha(pressed ? baseAlpha * 0.6f : baseAlpha);
    }
}
