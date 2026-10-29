package space.nimbusgo.nativekit

import android.content.Context
import android.graphics.Typeface
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.FrameLayout
import android.widget.TextView
import kotlin.math.max
import kotlin.math.roundToInt

/**
 * A navigation stack: a Material-style top bar with a back arrow and the top screen's title, and
 * the screens below it, sliding in and out. Screens' content is Go-rendered; going back by the
 * arrow or the system back button tells Go.
 */
internal class NNStack(c: Context, private val r: Renderer, private val stackTag: Int) : ViewGroup(c) {
    private val d = c.resources.displayMetrics.density
    private val bar = FrameLayout(c)
    private val title = TextView(c)
    private val back = TextView(c)
    private val screens = mutableListOf<View>()
    private val titles = HashMap<View, String>()
    private val leaving = HashSet<View>()
    private var reportedW = 0
    private var reportedH = 0

    init {
        bar.setBackgroundColor(0xffffffff.toInt())
        bar.elevation = 4 * d
        back.text = "←"
        back.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 24f)
        back.setTextColor(0xff111827.toInt())
        back.gravity = Gravity.CENTER
        back.contentDescription = "Back"
        back.setOnClickListener { goBack() }
        title.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 20f)
        title.typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
        title.setTextColor(0xff111827.toInt())
        title.gravity = Gravity.CENTER_VERTICAL
        title.isSingleLine = true
        bar.addView(back, FrameLayout.LayoutParams(px(56f), px(56f)))
        bar.addView(title, FrameLayout.LayoutParams(-1, px(56f)))
        addView(bar)
    }

    /** Bar colors from the theme. */
    fun style(bar: Int, text: Int, border: Int) {
        this.bar.setBackgroundColor(bar)
        title.setTextColor(text)
        back.setTextColor(text)
    }

    /** Shows exactly these screens, animating a push or pop of one. */
    fun setScreens(wanted: List<View>) {
        val n = screens.size
        when {
            wanted.size == n + 1 && wanted.subList(0, n) == screens -> {
                val incoming = wanted[n]
                screens.add(incoming)
                attach(incoming)
                if (n > 0 && width > 0) slide(screens[n - 1], incoming, push = true)
            }
            wanted.size == n - 1 && screens.subList(0, n - 1) == wanted -> {
                val out = screens.removeAt(n - 1)
                if (width > 0) slide(screens[n - 2], out, push = false) else detach(out)
            }
            wanted != screens -> {
                screens.filter { it !in wanted }.forEach(::detach)
                screens.clear()
                screens.addAll(wanted)
                wanted.forEach(::attach)
                screens.forEachIndexed { i, v -> v.visibility = if (i == screens.lastIndex) VISIBLE else INVISIBLE }
            }
        }
        updateBar()
    }

    fun setTitle(screen: View, t: String) {
        titles[screen] = t
        updateBar()
    }

    /** True if the view is a screen sliding out: it must stay until the animation ends. */
    fun isLeaving(v: View) = v in leaving

    /** The back arrow or system back: pops natively, then tells Go. */
    fun goBack(): Boolean {
        if (screens.size < 2) return false
        val rest = screens.subList(0, screens.size - 1).toList()
        setScreens(rest)
        r.popped(stackTag, rest.size)
        return true
    }

    val depth: Int get() = screens.size

    private fun slide(under: View, top: View, push: Boolean) {
        val w = width.toFloat()
        under.visibility = VISIBLE
        if (push) {
            top.translationX = w
            top.animate().translationX(0f).setDuration(SLIDE_MS).start()
            under.animate().translationX(-w * 0.3f).setDuration(SLIDE_MS)
                .withEndAction { if (screens.indexOf(under) != screens.lastIndex) under.visibility = INVISIBLE }
                .start()
        } else {
            leaving.add(top)
            under.translationX = -w * 0.3f
            under.animate().translationX(0f).setDuration(SLIDE_MS).start()
            top.animate().translationX(w).setDuration(SLIDE_MS).withEndAction { detach(top) }.start()
        }
    }

    private fun attach(v: View) {
        if (v.parent === this) return
        (v.parent as? ViewGroup)?.removeView(v)
        addView(v, indexOfChild(bar)) // screens under the bar
        v.translationX = 0f
    }

    private fun detach(v: View) {
        leaving.remove(v)
        titles.remove(v)
        if (v.parent === this) removeView(v)
    }

    private fun updateBar() {
        val top = screens.lastOrNull()
        title.text = top?.let { titles[it] } ?: ""
        val canGoBack = screens.size > 1
        back.visibility = if (canGoBack) VISIBLE else GONE
        (title.layoutParams as FrameLayout.LayoutParams).leftMargin = px(if (canGoBack) 72f else 16f)
        title.requestLayout()
    }

    override fun onMeasure(wSpec: Int, hSpec: Int) {
        val w = MeasureSpec.getSize(wSpec)
        val h = MeasureSpec.getSize(hSpec)
        val barH = px(BAR_DP)
        bar.measure(exactly(w), exactly(barH))
        val contentH = max(0, h - barH)
        for (i in 0 until childCount) {
            val c = getChildAt(i)
            if (c !== bar) c.measure(exactly(w), exactly(contentH))
        }
        setMeasuredDimension(w, h)
        if (w > 0 && (w != reportedW || contentH != reportedH)) {
            reportedW = w
            reportedH = contentH
            post { r.resized(stackTag, reportedW, reportedH) }
        }
    }

    override fun onLayout(changed: Boolean, l: Int, t: Int, right: Int, bottom: Int) {
        val w = right - l
        val barH = px(BAR_DP)
        bar.layout(0, 0, w, barH)
        for (i in 0 until childCount) {
            val c = getChildAt(i)
            if (c !== bar) c.layout(0, barH, w, barH + c.measuredHeight)
        }
    }

    private fun px(dp: Float) = (dp * d).roundToInt()

    private companion object {
        const val BAR_DP = 56f
        const val SLIDE_MS = 250L
    }
}
