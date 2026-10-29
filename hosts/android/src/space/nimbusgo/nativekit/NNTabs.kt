package space.nimbusgo.nativekit

import android.content.Context
import android.graphics.Typeface
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.LinearLayout
import android.widget.TextView
import kotlin.math.max
import kotlin.math.roundToInt

/**
 * A tab bar: the tabs' Go-rendered root views above a Material-style bottom bar. Only the selected
 * tab is visible; the others stay mounted, keeping their state.
 */
internal class NNTabs(c: Context, private val r: Renderer, private val tabsTag: Int) : ViewGroup(c) {
    private val d = c.resources.displayMetrics.density
    private val bar = LinearLayout(c)
    private val tabs = mutableListOf<View>()
    private val items = HashMap<View, TextView>()
    private var selected = 0
    private var accent = ACCENT
    private var muted = 0xff6b7280.toInt()
    private var reportedW = 0
    private var reportedH = 0

    init {
        bar.orientation = LinearLayout.HORIZONTAL
        bar.setBackgroundColor(0xffffffff.toInt())
        bar.elevation = 8 * d
        addView(bar)
    }

    fun setTabs(wanted: List<View>) {
        tabs.filter { it !in wanted }.forEach { gone ->
            removeView(gone)
            items.remove(gone)?.let(bar::removeView)
        }
        tabs.clear()
        tabs.addAll(wanted)
        wanted.forEachIndexed { i, v ->
            if (v.parent !== this) {
                (v.parent as? ViewGroup)?.removeView(v)
                addView(v, indexOfChild(bar)) // tabs under the bar
            }
            val item = items.getOrPut(v) { newItem() }
            if (item.parent == null) bar.addView(item, LinearLayout.LayoutParams(0, -1, 1f))
            item.setOnClickListener { pick(i) }
        }
        show()
    }

    /** Bar colors from the theme. */
    fun style(barColor: Int, accent: Int, muted: Int) {
        bar.setBackgroundColor(barColor)
        this.accent = accent
        this.muted = muted
        show()
    }

    fun setItem(tab: View, title: String) {
        items.getOrPut(tab) { newItem() }.text = title
    }

    fun select(i: Int) {
        selected = i
        show()
    }

    private fun pick(i: Int) {
        if (i == selected) return
        select(i)
        r.selectedTab(tabsTag, i)
    }

    private fun show() {
        tabs.forEachIndexed { i, v ->
            v.visibility = if (i == selected) VISIBLE else INVISIBLE
            items[v]?.let { item ->
                val on = i == selected
                item.setTextColor(if (on) accent else muted)
                item.typeface = Typeface.create(Typeface.DEFAULT, if (on) Typeface.BOLD else Typeface.NORMAL)
                item.background = if (on) indicator() else null
            }
        }
    }

    private fun newItem() = TextView(context).apply {
        gravity = Gravity.CENTER
        setTextSize(TypedValue.COMPLEX_UNIT_DIP, 13f)
        isSingleLine = true
    }

    /** A short accent line under the selected tab's label. */
    private fun indicator() = object : android.graphics.drawable.Drawable() {
        private val paint = android.graphics.Paint(android.graphics.Paint.ANTI_ALIAS_FLAG).apply { color = accent }

        override fun draw(canvas: android.graphics.Canvas) {
            val b = bounds
            val w = 32 * d
            val h = 3 * d
            val bottom = b.bottom - 6 * d
            canvas.drawRoundRect(b.exactCenterX() - w / 2, bottom - h, b.exactCenterX() + w / 2, bottom, h / 2, h / 2, paint)
        }

        override fun setAlpha(alpha: Int) {
            paint.alpha = alpha
        }

        override fun setColorFilter(filter: android.graphics.ColorFilter?) {
            paint.colorFilter = filter
        }

        @Deprecated("Deprecated in Java")
        override fun getOpacity() = android.graphics.PixelFormat.TRANSLUCENT
    }

    override fun onMeasure(wSpec: Int, hSpec: Int) {
        val w = MeasureSpec.getSize(wSpec)
        val h = MeasureSpec.getSize(hSpec)
        val barH = (BAR_DP * d).roundToInt()
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
            post { r.resized(tabsTag, reportedW, reportedH) }
        }
    }

    override fun onLayout(changed: Boolean, l: Int, t: Int, right: Int, bottom: Int) {
        val w = right - l
        val h = bottom - t
        val barH = (BAR_DP * d).roundToInt()
        for (i in 0 until childCount) {
            val c = getChildAt(i)
            if (c === bar) c.layout(0, h - barH, w, h) else c.layout(0, 0, w, c.measuredHeight)
        }
    }

    private companion object {
        const val BAR_DP = 56f
        val ACCENT = 0xff2563eb.toInt()
    }
}
