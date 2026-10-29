package space.nimbusgo.nativekit

import android.content.Context
import android.content.res.Configuration
import android.view.View
import android.view.ViewGroup

/**
 * The root of a Nimbus Native app's UI: an Activity's content view. It starts the Go runtime and
 * shows what the app renders.
 *
 *     setContentView(NimbusView(this, NimbusSchema.COMBINED))
 */
class NimbusView(c: Context, schema: Long) : ViewGroup(c) {
    val renderer = Renderer(this)
    private var rootView: View? = null
    private var insetTop = 0
    private var insetBottom = 0
    private var insetLeft = 0
    private var insetRight = 0

    init {
        setBackgroundColor(0xfff9fafb.toInt())
        @Suppress("DEPRECATION")
        setOnApplyWindowInsetsListener { _, insets ->
            insetTop = insets.systemWindowInsetTop
            insetBottom = insets.systemWindowInsetBottom
            insetLeft = insets.systemWindowInsetLeft
            insetRight = insets.systemWindowInsetRight
            sendViewport()
            insets
        }
        renderer.start(schema)
    }

    internal fun setRoot(v: View?) {
        if (rootView === v) return
        rootView?.let { removeView(it) }
        rootView = v
        if (v != null) {
            (v.parent as? ViewGroup)?.removeView(v)
            addView(v)
        }
    }

    private fun sendViewport() {
        val dark = resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK == Configuration.UI_MODE_NIGHT_YES
        renderer.sendViewport(width, height, insetTop, insetBottom, insetLeft, insetRight, dark)
    }

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        requestApplyInsets()
    }

    override fun onSizeChanged(w: Int, h: Int, oldw: Int, oldh: Int) {
        super.onSizeChanged(w, h, oldw, oldh)
        post { sendViewport() }
    }

    override fun onMeasure(wSpec: Int, hSpec: Int) {
        rootView?.let {
            val f = renderer.frameOf(it)
            it.measure(exactly(f[2]), exactly(f[3]))
        }
        setMeasuredDimension(MeasureSpec.getSize(wSpec), MeasureSpec.getSize(hSpec))
    }

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        rootView?.let {
            val f = renderer.frameOf(it)
            it.layout(f[0], f[1], f[0] + f[2], f[1] + f[3])
        }
    }
}
