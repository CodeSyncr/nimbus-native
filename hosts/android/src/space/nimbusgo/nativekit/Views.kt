package space.nimbusgo.nativekit

import android.annotation.SuppressLint
import android.content.Context
import android.content.res.ColorStateList
import android.graphics.drawable.GradientDrawable
import android.view.MotionEvent
import android.view.View
import android.view.ViewGroup
import android.widget.FrameLayout
import android.widget.HorizontalScrollView
import android.widget.ProgressBar
import android.widget.ScrollView
import kotlin.math.min
import kotlin.math.roundToInt

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

    /** Reports long presses too (a long press is then not also a tap). */
    fun reportLongPress(on: Boolean, tag: Int) {
        if (on) setOnLongClickListener { r.longPressed(tag); true } else setOnLongClickListener(null)
        isLongClickable = on
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

    private val d = c.resources.displayMetrics.density
    private var refreshable = false
    private var refreshing = false
    private var pulled = 0f
    private var indicator: ProgressBar? = null
    private var accent = 0xff2563eb.toInt()

    fun setContent(wPx: Int, hPx: Int, horizontal: Boolean, report: Boolean) {
        var s = scroller
        if (s == null || horizontal != this.horizontal) {
            if (s != null) {
                s.removeView(content)
                removeView(s)
            }
            this.horizontal = horizontal
            s = if (horizontal) HorizontalScrollView(context) else PullScrollView(context)
            s.addView(content)
            scroller = s
            addView(s, 0) // under the refresh indicator
        }
        content.fixedW = wPx
        content.fixedH = hPx
        content.requestLayout()
        s.setOnScrollChangeListener(
            if (report) View.OnScrollChangeListener { v, x, y, _, _ -> r.scrolled(id, x, y, v.width, v.height) } else null,
        )
    }

    /** Pull to refresh: whether it is on, whether the indicator should show, and its color. */
    fun setRefresh(refreshable: Boolean, refreshing: Boolean, color: Int) {
        this.refreshable = refreshable && !horizontal
        accent = color
        indicator?.indeterminateTintList = ColorStateList.valueOf(color)
        if (refreshing != this.refreshing) {
            this.refreshing = refreshing
            if (refreshing) settle(REST_DP * d) else settle(-SIZE_DP * d, hide = true)
        }
    }

    private fun indicator(): ProgressBar = indicator ?: ProgressBar(context).apply {
        isIndeterminate = true
        indeterminateTintList = ColorStateList.valueOf(accent)
        val pad = (8 * d).roundToInt()
        setPadding(pad, pad, pad, pad)
        background = GradientDrawable().apply {
            shape = GradientDrawable.OVAL
            setColor(0xffffffff.toInt())
        }
        elevation = 6 * d
        visibility = GONE
        indicator = this
        this@NNScroll.addView(this)
    }

    /** Where the indicator sits: its top edge, in px from the scroll view's top. */
    private fun place(y: Float) {
        val ind = indicator()
        ind.visibility = VISIBLE
        ind.translationY = y
    }

    private fun settle(y: Float, hide: Boolean = false) {
        val ind = indicator ?: if (hide) return else indicator()
        ind.visibility = VISIBLE
        ind.animate().translationY(y).setDuration(180).withEndAction { if (hide) ind.visibility = GONE }.start()
    }

    /** A ScrollView that, already at the top, turns a downward drag into a pull. */
    @SuppressLint("ViewConstructor")
    private inner class PullScrollView(c: Context) : ScrollView(c) {
        private var startY = 0f
        private var pulling = false

        override fun onInterceptTouchEvent(e: MotionEvent): Boolean {
            if (e.actionMasked == MotionEvent.ACTION_DOWN) {
                startY = e.y
                pulling = false
            }
            return super.onInterceptTouchEvent(e)
        }

        @SuppressLint("ClickableViewAccessibility")
        override fun onTouchEvent(e: MotionEvent): Boolean {
            if (!refreshable || refreshing) return super.onTouchEvent(e)
            when (e.actionMasked) {
                MotionEvent.ACTION_DOWN -> startY = e.y
                MotionEvent.ACTION_MOVE -> {
                    val dy = e.y - startY
                    if (scrollY == 0 && dy > 0) {
                        pulling = true
                        pulled = dy / 2 // resistance
                        place(min(pulled, MAX_DP * d) - SIZE_DP * d)
                        return true
                    }
                    if (pulling && dy <= 0) {
                        pulling = false
                        settle(-SIZE_DP * d, hide = true)
                    }
                }
                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> if (pulling) {
                    pulling = false
                    if (pulled >= TRIGGER_DP * d && e.actionMasked == MotionEvent.ACTION_UP) {
                        refreshing = true
                        settle(REST_DP * d)
                        r.refreshPulled(this@NNScroll.id)
                    } else {
                        settle(-SIZE_DP * d, hide = true)
                    }
                    return true
                }
            }
            return super.onTouchEvent(e)
        }
    }

    override fun onMeasure(wSpec: Int, hSpec: Int) {
        val w = MeasureSpec.getSize(wSpec)
        val h = MeasureSpec.getSize(hSpec)
        scroller?.measure(exactly(w), exactly(h))
        val size = (SIZE_DP * d).roundToInt()
        indicator?.measure(exactly(size), exactly(size))
        setMeasuredDimension(w, h)
    }

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        scroller?.layout(0, 0, r - l, b - t)
        indicator?.let {
            val size = (SIZE_DP * d).roundToInt()
            val x = (r - l - size) / 2
            it.layout(x, 0, x + size, size)
        }
    }

    private companion object {
        const val SIZE_DP = 40f // the indicator's circle
        const val REST_DP = 16f // where it waits while refreshing
        const val TRIGGER_DP = 72f // how far to pull
        const val MAX_DP = 110f
    }
}

internal fun exactly(px: Int) = View.MeasureSpec.makeMeasureSpec(px, View.MeasureSpec.EXACTLY)
