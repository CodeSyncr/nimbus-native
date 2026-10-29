package space.nimbusgo.nativekit

import android.content.Context
import android.view.View
import android.view.ViewGroup
import android.widget.FrameLayout
import android.widget.HorizontalScrollView
import android.widget.ScrollView

/**
 * A container that places each child at the frame the Go layout computed (in px, kept by the
 * renderer). Children may draw outside it unless the box style clips.
 */
internal open class NNGroup(c: Context, val r: Renderer) : ViewGroup(c) {
    /** Fixed size for a scroll view's content, which its scroller measures without limits; -1 if unset. */
    var fixedW = -1
    var fixedH = -1

    init {
        clipChildren = false
        clipToPadding = false
    }

    override fun onMeasure(wSpec: Int, hSpec: Int) {
        for (i in 0 until childCount) {
            val child = getChildAt(i)
            val f = r.frameOf(child)
            child.measure(exactly(f[2]), exactly(f[3]))
        }
        setMeasuredDimension(
            if (fixedW >= 0) fixedW else MeasureSpec.getSize(wSpec),
            if (fixedH >= 0) fixedH else MeasureSpec.getSize(hSpec),
        )
    }

    override fun onLayout(changed: Boolean, l: Int, t: Int, right: Int, bottom: Int) {
        for (i in 0 until childCount) {
            val child = getChildAt(i)
            val f = r.frameOf(child)
            child.layout(f[0], f[1], f[0] + f[2], f[1] + f[3])
        }
    }

    override fun shouldDelayChildPressedState() = false
}

/** A container that reports taps and dims while pressed. */
internal class NNPressable(c: Context, r: Renderer, tag: Int) : NNGroup(c, r) {
    var baseAlpha = 1f

    init {
        isClickable = true
        setOnClickListener { r.press(tag) }
    }

    override fun setPressed(pressed: Boolean) {
        super.setPressed(pressed)
        alpha = if (pressed) baseAlpha * 0.6f else baseAlpha
    }
}

/**
 * A scroll view whose children go in [content], sized to the content size the layout reports.
 * The platform scroller (vertical or horizontal) is made on the first Scroll op.
 */
internal class NNScroll(c: Context, r: Renderer) : NNGroup(c, r) {
    val content = NNGroup(c, r)
    var scroller: FrameLayout? = null
        private set
    private var horizontal = false

    init {
        // Clipping lets the renderer skip rows outside the visible area; the
        // content is sized to hold all its children, so nothing is cut off.
        content.clipChildren = true
        clipChildren = true // content must not draw outside the scroll view
    }

    fun setContent(wPx: Int, hPx: Int, horizontal: Boolean, report: Boolean) {
        var s = scroller
        if (s == null || horizontal != this.horizontal) {
            if (s != null) {
                s.removeView(content)
                removeView(s)
            }
            this.horizontal = horizontal
            s = if (horizontal) HorizontalScrollView(context) else ScrollView(context)
            s.addView(content)
            scroller = s
            addView(s)
        }
        content.fixedW = wPx
        content.fixedH = hPx
        content.requestLayout()
        s.setOnScrollChangeListener(
            if (report) View.OnScrollChangeListener { v, x, y, _, _ -> r.scrolled(id, x, y, v.width, v.height) } else null,
        )
    }

    override fun onMeasure(wSpec: Int, hSpec: Int) {
        val w = MeasureSpec.getSize(wSpec)
        val h = MeasureSpec.getSize(hSpec)
        scroller?.measure(exactly(w), exactly(h))
        setMeasuredDimension(w, h)
    }

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        scroller?.layout(0, 0, r - l, b - t)
    }
}

internal fun exactly(px: Int) = View.MeasureSpec.makeMeasureSpec(px, View.MeasureSpec.EXACTLY)
