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
    private val modals = mutableListOf<NNModal>()
    private var insetTop = 0
    private var insetBottom = 0
    private var insetLeft = 0
    private var insetRight = 0
    private var imeBottom = 0 // the keyboard's height from the window's bottom

    init {
        setBackgroundColor(0xfff9fafb.toInt())
        @Suppress("DEPRECATION")
        setOnApplyWindowInsetsListener { _, insets ->
            insetTop = insets.systemWindowInsetTop
            insetBottom = insets.systemWindowInsetBottom
            insetLeft = insets.systemWindowInsetLeft
            insetRight = insets.systemWindowInsetRight
            imeBottom = 0
            if (android.os.Build.VERSION.SDK_INT >= 30) {
                // The keyboard is its own inset; the bars' bottom inset is the navigation bar.
                imeBottom = insets.getInsets(android.view.WindowInsets.Type.ime()).bottom
                insetBottom = insets.getInsets(android.view.WindowInsets.Type.systemBars()).bottom
            }
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

    /** The theme's background here, and its bar color on the system bars. */
    internal fun applyChrome(c: Render.Chrome) {
        setBackgroundColor(Renderer.argb(c.background))
        val window = (context as? android.app.Activity)?.window ?: return
        @Suppress("DEPRECATION")
        run {
            window.statusBarColor = Renderer.argb(c.bar)
            window.navigationBarColor = Renderer.argb(c.bar)
            var flags = window.decorView.systemUiVisibility
            val lightBars = View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR or View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR
            flags = if (c.dark) flags and lightBars.inv() else flags or lightBars
            window.decorView.systemUiVisibility = flags
        }
    }

    internal fun present(m: NNModal) {
        if (m in modals) return
        (m.parent as? ViewGroup)?.removeView(m)
        modals.add(m)
        addView(m) // above the app and earlier modals
        m.slideIn()
    }

    internal fun dismiss(m: NNModal) {
        modals.remove(m)
        m.slideOut { removeView(m) }
    }

    /** The modal on top, if any. */
    internal val topModal: NNModal? get() = modals.lastOrNull()

    private fun sendViewport() {
        val dark = resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK == Configuration.UI_MODE_NIGHT_YES
        renderer.sendViewport(width, height, insetTop, insetBottom, insetLeft, insetRight, dark, keyboardOverlap())
    }

    /** How far the keyboard covers this view's bottom. When the window already shrank for it
     *  (adjustResize), this view ends above the keyboard and the overlap is 0. */
    private fun keyboardOverlap(): Int {
        if (imeBottom == 0) return 0
        val loc = IntArray(2)
        getLocationInWindow(loc)
        val windowH = super.getRootView().height
        return maxOf(0, loc[1] + height - (windowH - imeBottom))
    }

    // Activities declare configChanges (rotation, dark mode, size), so they come here instead
    // of being created again: the viewport carries the change to Go.
    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        post { sendViewport() }
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
        val w = MeasureSpec.getSize(wSpec)
        val h = MeasureSpec.getSize(hSpec)
        rootView?.let {
            val f = renderer.frameOf(it)
            it.measure(exactly(f[2]), exactly(f[3]))
        }
        for (i in 0 until childCount) {
            val c = getChildAt(i)
            if (c is NNModal) c.measure(exactly(w), exactly(h)) // modals fill the screen
        }
        setMeasuredDimension(w, h)
    }

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        rootView?.let {
            val f = renderer.frameOf(it)
            it.layout(f[0], f[1], f[0] + f[2], f[1] + f[3])
        }
        for (i in 0 until childCount) {
            val c = getChildAt(i)
            if (c is NNModal) c.layout(0, 0, r - l, b - t)
        }
    }
}
