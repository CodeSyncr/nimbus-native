package space.nimbusgo.nativekit

import android.content.Context
import android.content.res.ColorStateList
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Paint
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.text.Editable
import android.text.InputType
import android.text.Layout
import android.text.StaticLayout
import android.text.TextPaint
import android.text.TextUtils
import android.text.TextWatcher
import android.util.Log
import android.util.LruCache
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.ProgressBar
import android.widget.TextView
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.Executors
import kotlin.math.ceil
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt

/**
 * Applies the Go runtime's commits to Android views and reports taps, edits, scrolling and the
 * viewport back. Lengths arrive in dp and are kept as px frames.
 */
class Renderer internal constructor(private val root: NimbusView) {
    /** After each commit is applied: the commit and how long applying took, in ms. */
    fun interface CommitListener {
        fun onCommit(c: Render.Commit, applyMs: Double)
    }

    var onCommit: CommitListener? = null

    private val ctx: Context = root.context
    private val density = ctx.resources.displayMetrics.density
    private val views = HashMap<Int, View>()
    private val frames = HashMap<Int, IntArray>() // l, t, w, h in px
    private val pendingTitles = HashMap<View, String>() // titles for screens not in a stack yet
    private var rootTag = 0

    /** The theme for what Android draws itself (bars, backgrounds); see Chrome. */
    internal var chrome = Render.Chrome()
        private set
    private var lastViewport: Render.Viewport? = null
    private var started = false

    internal fun start(schema: Long) {
        Plugins.context = ctx.applicationContext
        // Where a view is in the window, for what opens beside it (ui.Menu).
        Plugins.register("nimbusView", Plugins.Handler { call ->
            val v = views[call.args.optInt("tag")] ?: return@Handler call.reject(Plugins.NOT_FOUND, "no such view")
            val at = IntArray(2)
            v.getLocationInWindow(at)
            call.resolve(mapOf("X" to at[0] / density, "Y" to at[1] / density, "W" to v.width / density, "H" to v.height / density))
        })
        Plugins.activity = ctx as? android.app.Activity
        Plugins.boot()
        Bridge.setListener { bytes ->
            val m = try {
                Render.decode(bytes)
            } catch (e: WireException) {
                Log.e("NimbusNative", "undecodable commit", e)
                return@setListener
            }
            if (m is Render.Commit) apply(m)
        }
        val d = density
        Bridge.setHostHandler { req -> answer(req, d) }
        val fresh = Bridge.start(schema)
        started = true
        // A runtime already running drew into views that are gone: draw it all again here.
        if (!fresh) Bridge.send(Render.encode(Render.Reattach(reason = "activity created again")))
    }

    // ---- viewport and events ----

    internal fun sendViewport(wPx: Int, hPx: Int, top: Int, bottom: Int, left: Int, right: Int, dark: Boolean, keyboardPx: Int = 0) {
        if (!started || wPx == 0) return
        val v = Render.Viewport(
            w = wPx / density, h = hPx / density, scale = density,
            safeTop = top / density, safeBottom = bottom / density, safeLeft = left / density, safeRight = right / density,
            platform = "android", dark = dark, keyboard = keyboardPx / density,
        )
        if (v == lastViewport) return
        lastViewport = v
        Bridge.send(Render.encode(v))
    }

    /** Sends a press as if the user tapped the view (for tests and demos). */
    fun press(tag: Int) = Bridge.send(Render.encode(Render.Press(tag = tag.toLong())))

    /** Tells Go the user went back in a stack. */
    internal fun popped(tag: Int, depth: Int) =
        Bridge.send(Render.encode(Render.Popped(tag = tag.toLong(), depth = depth.toLong())))

    /** Opens a link to the app (myapp://orders/42): its page shows, pushed on the navigation
     *  stack on screen, or as a sheet. */
    fun open(url: String) = Bridge.send(Render.encode(Render.Open(url = url)))

    /** Tells Go about a long press on a pressable that asked for them. */
    internal fun longPressed(tag: Int) = Bridge.send(Render.encode(Render.LongPressed(tag = tag.toLong())))

    /** Tells Go about a drag, in points. */
    internal fun panned(tag: Int, phase: Int, x: Float, y: Float, vx: Float, vy: Float) = Bridge.send(
        Render.encode(Render.Panned(tag = tag.toLong(), phase = phase, x = x / density, y = y / density, vx = vx / density, vy = vy / density)),
    )

    /** Tells Go about a pinch. */
    internal fun pinched(tag: Int, phase: Int, scale: Float) =
        Bridge.send(Render.encode(Render.Pinched(tag = tag.toLong(), phase = phase, scale = scale)))

    /** Tells Go the user pulled a scroll view to refresh. */
    internal fun refreshPulled(tag: Int) = Bridge.send(Render.encode(Render.Refresh(tag = tag.toLong())))

    /** Tells Go the user picked a tab. */
    internal fun selectedTab(tag: Int, index: Int) =
        Bridge.send(Render.encode(Render.Selected(tag = tag.toLong(), index = index.toLong())))

    /** Tells Go the size a stack gives its screens (px → dp). */
    internal fun resized(tag: Int, w: Int, h: Int) =
        Bridge.send(Render.encode(Render.Resized(tag = tag.toLong(), w = w / density, h = h / density)))

    /** Reports a List's scroll position (px from the scroller) in dp. */
    internal fun scrolled(tag: Int, x: Int, y: Int, w: Int, h: Int) = Bridge.send(
        Render.encode(Render.Scrolled(tag = tag.toLong(), x = x / density, y = y / density, w = w / density, h = h / density)),
    )

    /**
     * The system back button: goes back in the deepest stack that can. Returns false when there is
     * nowhere to go back to, so the Activity can finish.
     */
    fun back(): Boolean {
        // The top modal first: pop its stack, else close it.
        root.topModal?.let { m ->
            val inModal = views.values.filterIsInstance<NNStack>().firstOrNull { it.depth > 1 && isInside(it, m) }
            if (inModal != null) return inModal.goBack()
            val tag = views.entries.first { it.value === m }.key
            root.dismiss(m)
            Bridge.send(Render.encode(Render.Dismissed(tag = tag.toLong())))
            return true
        }
        val stack = views.values.filterIsInstance<NNStack>().lastOrNull { it.isShown && it.depth > 1 }
        return stack?.goBack() ?: false
    }

    private fun isInside(v: View, ancestor: View): Boolean {
        var p = v.parent
        while (p != null) {
            if (p === ancestor) return true
            p = p.parent
        }
        return false
    }

    /** The tag of the view with this test ID, or 0. */
    fun tagForTestID(id: String): Int = views.entries.firstOrNull { it.value.tag == id }?.key ?: 0

    /** The view for a tag (a scroll view's platform scroller, for scroll views). */
    fun viewForTag(tag: Int): View? = views[tag].let { if (it is NNScroll) it.scroller else it }

    internal fun frameOf(child: View): IntArray = frames[child.id] ?: NO_FRAME

    // ---- applying commits ----

    private fun apply(c: Render.Commit) {
        val t0 = System.nanoTime()
        val moved = HashSet<View>()
        for (op in c.ops) {
            when (op) {
                is Render.Create -> create(op)
                is Render.Destroy -> destroy(op)
                is Render.Children -> children(op)
                is Render.Frame -> {
                    val tag = op.tag.toInt()
                    val f = intArrayOf(
                        (op.x * density).roundToInt(), (op.y * density).roundToInt(),
                        (op.w * density).roundToInt(), (op.h * density).roundToInt(),
                    )
                    val old = frames[tag]
                    frames[tag] = f
                    val v = views[tag]
                    v?.let(moved::add)
                    // Style.Transition: slide from where it was to where layout puts it.
                    val ms = transitions[tag] ?: 0L
                    if (v != null && ms > 0 && old != null && (old[0] != f[0] || old[1] != f[1])) {
                        v.translationX += (old[0] - f[0]).toFloat()
                        v.translationY += (old[1] - f[1]).toFloat()
                        v.animate().translationX(0f).translationY(0f).setDuration(ms).start()
                    }
                }
                is Render.Slider -> (views[op.tag.toInt()] as? NNSlider)?.apply(op)
                is Render.Native -> {
                    val tag = op.tag
                    (views[tag.toInt()] as? NNNativeHost)?.set(op) { name, data ->
                        Bridge.send(Render.encode(Render.NativeEvent(tag = tag, name = name, data = data)))
                    }
                }
                is Render.Draw -> (views[op.tag.toInt()] as? NNCanvas)?.set(op)
                is Render.Box -> box(op)
                is Render.Text -> (views[op.tag.toInt()] as? TextView)?.let { text(it, op) }
                is Render.Image -> (views[op.tag.toInt()] as? NNImage)?.load(op)
                is Render.Input -> (views[op.tag.toInt()] as? NNInput)?.apply(op, density)
                is Render.Scroll -> (views[op.tag.toInt()] as? NNScroll)?.let {
                    it.setContent((op.w * density).roundToInt(), (op.h * density).roundToInt(), op.horizontal, op.report, op.stickToEnd)
                    it.setRefresh(op.refreshable, op.refreshing, argb(if (chrome.accent != 0L) chrome.accent else 0x2563ebffL))
                }
                is Render.Switch -> (views[op.tag.toInt()] as? NNSwitch)?.apply(op)
                is Render.Spinner -> (views[op.tag.toInt()] as? ProgressBar)?.let {
                    if (op.color != 0L) it.indeterminateTintList = ColorStateList.valueOf(argb(op.color))
                }
                is Render.Screen -> {
                    val v = views[op.tag.toInt()]
                    val stack = v?.parent as? NNStack
                    if (stack != null) stack.setTitle(v, op.title) else if (v != null) pendingTitles[v] = op.title
                }
                is Render.TabItem -> {
                    val v = views[op.tag.toInt()]
                    (v?.parent as? NNTabs)?.setItem(v, op.title, op.icon)
                }
                is Render.TabSelect -> (views[op.tag.toInt()] as? NNTabs)?.select(op.index.toInt())
                is Render.Present -> (views[op.tag.toInt()] as? NNModal)?.let(root::present)
                is Render.Chrome -> applyChrome(op)
                null -> {}
            }
        }
        if (c.root != 0L) {
            rootTag = c.root.toInt()
            root.setRoot(views[rootTag])
        }
        for (v in moved) {
            v.requestLayout()
            (v.parent as? View)?.requestLayout()
        }
        onCommit?.onCommit(c, (System.nanoTime() - t0) / 1e6)
    }

    private fun create(o: Render.Create) {
        val tag = o.tag.toInt()
        val v: View = when (o.kind) {
            Render.Kind.TEXT -> TextView(ctx).apply {
                setPadding(0, 0, 0, 0)
                breakStrategy = Layout.BREAK_STRATEGY_SIMPLE
                hyphenationFrequency = Layout.HYPHENATION_FREQUENCY_NONE
                setTextColor(0xff111111.toInt())
            }
            Render.Kind.IMAGE -> NNImage(ctx)
            Render.Kind.PRESSABLE -> NNPressable(ctx, this, tag)
            Render.Kind.SCROLL -> NNScroll(ctx, this)
            Render.Kind.INPUT -> NNInput(ctx, tag, density)
            Render.Kind.STACK -> NNStack(ctx, this, tag)
            Render.Kind.TABS -> NNTabs(ctx, this, tag)
            Render.Kind.MODAL -> NNModal(ctx, this, tag)
            Render.Kind.SPINNER -> ProgressBar(ctx).apply { isIndeterminate = true }
            Render.Kind.SWITCH -> NNSwitch(ctx, tag)
            Render.Kind.SLIDER -> NNSlider(ctx, tag)
            Render.Kind.CANVAS -> NNCanvas(ctx)
            Render.Kind.NATIVE -> NNNativeHost(ctx)
            else -> NNGroup(ctx, this)
        }
        v.id = tag
        views[tag] = v
        style(v)
    }

    private fun applyChrome(c: Render.Chrome) {
        chrome = c
        root.applyChrome(c)
        views.values.forEach(::style)
    }

    /** Theme what the platform draws: bars, modal backgrounds, placeholders. */
    private fun style(v: View) {
        val c = chrome
        if (c.bar == 0L) return
        when (v) {
            is NNStack -> v.style(argb(c.bar), argb(c.barText), argb(c.border))
            is NNTabs -> v.style(argb(c.bar), argb(c.accent), argb(c.textMuted))
            is NNModal -> v.setBackgroundColor(argb(c.background))
            is NNInput -> v.setHintTextColor(argb(c.textMuted))
        }
    }

    /**
     * Detach only the top of each destroyed subtree: its descendants go with it. A screen sliding
     * out of a stack goes when its animation ends.
     */
    private fun destroy(o: Render.Destroy) {
        val gone = o.tags.mapNotNullTo(HashSet()) { views[it.toInt()] }
        for (tag in o.tags) {
            val v = views.remove(tag.toInt())
            frames.remove(tag.toInt())
            (v?.background as? BoxDrawable)?.detach()
            hidden.remove(tag.toInt())
            if (v is NNModal) {
                if (!v.leaving) root.dismiss(v) // slides away, then goes
                continue
            }
            val parent = v?.parent as? ViewGroup ?: continue
            if (parent in gone || contentOwner(parent) in gone) continue
            if (parent is NNStack && parent.isLeaving(v)) continue
            parent.removeView(v)
        }
    }

    /** Makes a view's managed children exactly these, in order. */
    private fun children(o: Render.Children) {
        val pv = views[o.tag.toInt()] ?: return
        val wanted = o.children.mapNotNull { views[it.toInt()] }
        if (pv is NNStack) {
            pv.setScreens(wanted)
            for (v in wanted) pendingTitles.remove(v)?.let { pv.setTitle(v, it) }
            return
        }
        if (pv is NNTabs) {
            pv.setTabs(wanted)
            return
        }
        val parent = if (pv is NNScroll) pv.content else pv as ViewGroup
        val keep = wanted.toHashSet()
        for (i in parent.childCount - 1 downTo 0) {
            if (parent.getChildAt(i) !in keep) parent.removeViewAt(i)
        }
        wanted.forEachIndexed { i, v ->
            if (i < parent.childCount && parent.getChildAt(i) === v) return@forEachIndexed
            (v.parent as? ViewGroup)?.removeView(v)
            parent.addView(v, i)
        }
    }

    private val transitions = HashMap<Int, Long>() // ms, for views whose changes animate

    private val hidden = HashSet<Int>() // tags hidden by Style.Hidden

    private fun box(o: Render.Box) {
        val v = views[o.tag.toInt()] ?: return
        val ms = o.transition
        if (ms > 0) transitions[o.tag.toInt()] = ms else transitions.remove(o.tag.toInt())
        if (v is NNPressable) v.reportLongPress(o.longPress, o.tag.toInt())
        (v as? NNGroup)?.setGestures(o.pan, o.pinch, o.tag.toInt())
        (v.background as? BoxDrawable)?.detach()
        val drawn = o.background != 0L || o.radius != 0f || o.borderWidth != 0f || o.gradientColors.size >= 2 ||
            o.blur != Render.Blur.NONE || o.shadowColor != 0L
        if (drawn) {
            v.background = BoxDrawable(v, o, density)
        } else if (v !is ProgressBar) {
            v.background = null
        }

        if (v is NNPressable) v.baseAlpha = o.opacity
        val scale = if (o.scale == 0f) 1f else o.scale
        if (ms > 0 && v.isAttachedToWindow) {
            v.animate().alpha(o.opacity).scaleX(scale).scaleY(scale).rotation(o.rotate).setDuration(ms).start()
        } else {
            v.alpha = o.opacity
            v.scaleX = scale
            v.scaleY = scale
            v.rotation = o.rotate
        }
        v.clipToOutline = o.clip
        // Only a change of Style.Hidden touches visibility: stacks and tabs set it on their screens.
        if (o.hidden) {
            if (hidden.add(o.tag.toInt())) v.visibility = View.INVISIBLE
        } else if (hidden.remove(o.tag.toInt())) {
            v.visibility = View.VISIBLE
        }
        if (v is NNGroup && v !is NNScroll) v.clipChildren = o.clip // scroll views always clip
        v.contentDescription = o.label.ifEmpty { null }
        v.tag = o.testID.ifEmpty { null }
    }

    private fun text(t: TextView, o: Render.Text) {
        t.text = if (o.spans.isEmpty()) o.text else rich(o.spans, density, ctx)
        t.setTextSize(TypedValue.COMPLEX_UNIT_PX, o.font.size * density)
        t.typeface = fontFor(o.font, ctx)
        applyExtras(t.paint, o.font, density)
        t.letterSpacing = t.paint.letterSpacing
        t.fontFeatureSettings = t.paint.fontFeatureSettings
        t.setLineSpacing(lineAdd(t.paint, o.font, density), 1f)
        if (o.color != 0L) t.setTextColor(argb(o.color))
        val g = when (o.align) {
            Render.Align.CENTER -> Gravity.CENTER_HORIZONTAL
            Render.Align.END -> Gravity.END
            else -> Gravity.START
        }
        t.gravity = g or Gravity.TOP
        if (o.lines > 0) {
            t.maxLines = o.lines.toInt()
            t.ellipsize = TextUtils.TruncateAt.END
        } else {
            t.maxLines = Int.MAX_VALUE
            t.ellipsize = null
        }
        // Shrink to fit (Style.MinScale): one line, smaller rather than cut.
        if (o.minScale > 0f && Build.VERSION.SDK_INT >= 26) {
            val max = (o.font.size * density).roundToInt().coerceAtLeast(2)
            val min = (max * o.minScale).roundToInt().coerceIn(1, max - 1)
            t.setAutoSizeTextTypeUniformWithPresetSizes(IntArray(max - min + 1) { min + it }, TypedValue.COMPLEX_UNIT_PX)
        } else if (Build.VERSION.SDK_INT >= 26) {
            t.setAutoSizeTextTypeWithDefaults(TextView.AUTO_SIZE_TEXT_TYPE_NONE)
        }
    }

    /** An image view that loads its picture from a URL, with a shared cache. */
    private class NNImage(c: Context) : ImageView(c) {
        private var url: String? = null

        fun load(o: Render.Image) {
            scaleType = when (o.fit) {
                Render.Fit.CONTAIN -> ScaleType.FIT_CENTER
                Render.Fit.STRETCH -> ScaleType.FIT_XY
                else -> ScaleType.CENTER_CROP
            }
            val u = o.url
            if (u == url) return
            url = u
            setImageDrawable(null)
            bitmaps.get(u)?.let {
                setImageBitmap(it)
                return
            }
            loader.execute {
                try {
                    val conn = URL(u).openConnection() as HttpURLConnection
                    conn.instanceFollowRedirects = true
                    val b = conn.inputStream.use { BitmapFactory.decodeStream(it) } ?: return@execute
                    bitmaps.put(u, b)
                    main.post { if (u == url) setImageBitmap(b) }
                } catch (e: Exception) {
                    Log.w("NimbusNative", "image $u: $e")
                }
            }
        }
    }

    /** A single-line text field that reports edits (but not the runtime's own updates). */
    private class NNInput(c: Context, tag: Int, density: Float) : EditText(c) {
        private var applying = false

        private var multiline = false
        private var submits = false
        private var focusedOnce = false

        init {
            isSingleLine = true
            // The keyboard's action key (Send) on an input that submits.
            setOnEditorActionListener { _, action, event ->
                val enter = event?.keyCode == android.view.KeyEvent.KEYCODE_ENTER && event.action == android.view.KeyEvent.ACTION_DOWN
                if (submits && (action == android.view.inputmethod.EditorInfo.IME_ACTION_SEND || enter)) {
                    Bridge.send(Render.encode(Render.Submitted(tag = tag.toLong())))
                    true
                } else {
                    false
                }
            }
            background = null
            val pad = (10 * density).roundToInt() // the runtime measures inputs with 10 dp padding
            setPadding(pad, pad, pad, pad)
            addTextChangedListener(object : TextWatcher {
                override fun beforeTextChanged(s: CharSequence?, a: Int, b: Int, n: Int) {}

                override fun onTextChanged(s: CharSequence?, a: Int, b: Int, n: Int) {}

                override fun afterTextChanged(s: Editable) {
                    if (!applying) Bridge.send(Render.encode(Render.Changed(tag = tag.toLong(), text = s.toString())))
                }
            })
        }

        fun apply(o: Render.Input, d: Float) {
            applying = true
            if (text.toString() != o.value) {
                setText(o.value)
                setSelection(o.value.length)
            }
            applying = false
            hint = o.placeholder.ifEmpty { null }
            setTextSize(TypedValue.COMPLEX_UNIT_PX, o.font.size * d)
            typeface = fontFor(o.font, context)
            if (o.color != 0L) setTextColor(argb(o.color))
            var type = when (o.keyboard) {
                Render.Keyboard.NUMBER -> InputType.TYPE_CLASS_NUMBER or InputType.TYPE_NUMBER_FLAG_DECIMAL
                Render.Keyboard.EMAIL -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_EMAIL_ADDRESS
                Render.Keyboard.URL -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI
                else -> InputType.TYPE_CLASS_TEXT
            }
            if (o.secure) type = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
            if (o.multiline) type = type or InputType.TYPE_TEXT_FLAG_MULTI_LINE
            if (inputType != type) inputType = type
            if (o.multiline != multiline) {
                multiline = o.multiline
                isSingleLine = !multiline
                gravity = if (multiline) Gravity.TOP or Gravity.START else Gravity.CENTER_VERTICAL or Gravity.START
            }
            submits = o.submit
            imeOptions = if (o.submit) android.view.inputmethod.EditorInfo.IME_ACTION_SEND else android.view.inputmethod.EditorInfo.IME_ACTION_UNSPECIFIED
            if (o.autoFocus && !focusedOnce) {
                focusedOnce = true
                post {
                    requestFocus()
                    val imm = context.getSystemService(Context.INPUT_METHOD_SERVICE) as android.view.inputmethod.InputMethodManager
                    imm.showSoftInput(this, android.view.inputmethod.InputMethodManager.SHOW_IMPLICIT)
                }
            }
        }
    }

    /** A slider: a SeekBar with 1000 steps over the range; reports the user's drags. */
    private class NNSlider(c: Context, tag: Int) : android.widget.SeekBar(c) {
        private var min = 0f
        private var max = 1f
        private var applying = false

        init {
            setMax(STEPS)
            setOnSeekBarChangeListener(object : OnSeekBarChangeListener {
                override fun onProgressChanged(s: android.widget.SeekBar, p: Int, fromUser: Boolean) {
                    if (fromUser && !applying) {
                        Bridge.send(Render.encode(Render.Slid(tag = tag.toLong(), value = min + (max - min) * p / STEPS)))
                    }
                }

                override fun onStartTrackingTouch(s: android.widget.SeekBar) {}

                override fun onStopTrackingTouch(s: android.widget.SeekBar) {}
            })
        }

        fun apply(o: Render.Slider) {
            min = o.min
            max = if (o.max > o.min) o.max else o.min + 1
            applying = true
            val p = ((o.value - min) / (max - min) * STEPS).roundToInt().coerceIn(0, STEPS)
            if (progress != p) progress = p
            applying = false
            isEnabled = o.enabled
            if (o.tint != 0L) {
                val t = ColorStateList.valueOf(argb(o.tint))
                progressTintList = t
                thumbTintList = t
            }
        }

        private companion object {
            const val STEPS = 1000
        }
    }

    /** An on/off switch; reports the user's flips, not the renderer's. */
    private class NNSwitch(c: Context, tag: Int) : android.widget.Switch(c) {
        private var applying = false

        init {
            setOnCheckedChangeListener { _, on ->
                if (!applying) Bridge.send(Render.encode(Render.Toggled(tag = tag.toLong(), on = on)))
            }
        }

        fun apply(o: Render.Switch) {
            applying = true
            if (isChecked != o.on) isChecked = o.on
            applying = false
            isEnabled = o.enabled
            if (o.tint != 0L) {
                val on = argb(o.tint)
                val states = arrayOf(intArrayOf(android.R.attr.state_checked), intArrayOf())
                thumbTintList = ColorStateList(states, intArrayOf(on, 0xfff5f5f5.toInt()))
                trackTintList = ColorStateList(states, intArrayOf((on and 0x00ffffff) or 0x80000000.toInt(), 0x40000000))
            }
        }
    }

    internal companion object {
        val NO_FRAME = intArrayOf(0, 0, 0, 0)
        val loader = Executors.newFixedThreadPool(4)!!
        val main = Handler(Looper.getMainLooper())
        val bitmaps = object : LruCache<String, Bitmap>(16 shl 20) {
            override fun sizeOf(key: String, value: Bitmap) = value.byteCount
        }

        /** Answers Go's synchronous calls (on a Go thread): text measurement, and starting plugin calls. */
        fun answer(req: ByteArray, d: Float): ByteArray? = when (val m = Render.decode(req)) {
            is Render.MeasureText -> measure(m, d)
            is Render.PluginCall -> Render.encode(Plugins.start(m))
            else -> null
        }

        /** Measures text the way TextView draws it. */
        private fun measure(mt: Render.MeasureText, d: Float): ByteArray {
            val p = TextPaint(Paint.ANTI_ALIAS_FLAG).apply {
                textSize = mt.font.size * d
                typeface = fontFor(mt.font, Plugins.context)
            }
            applyExtras(p, mt.font, d)
            val text: CharSequence = if (mt.spans.isEmpty()) mt.text else rich(mt.spans, d, Plugins.context)
            val desired = ceil(Layout.getDesiredWidth(text, p)).toInt()
            // Round, don't truncate: a width Go got from an earlier measurement
            // converts back to 99.9999 px, and truncating that loses a pixel and
            // wraps the text onto a second line.
            val width = if (mt.maxWidth < 0) desired else min(desired, (mt.maxWidth * d).roundToInt())
            val b = StaticLayout.Builder.obtain(text, 0, text.length, p, max(width, 1))
                .setIncludePad(true)
                .setBreakStrategy(Layout.BREAK_STRATEGY_SIMPLE)
                .setHyphenationFrequency(Layout.HYPHENATION_FREQUENCY_NONE)
                .setLineSpacing(lineAdd(p, mt.font, d), 1f)
            if (mt.lines > 0) b.setMaxLines(mt.lines.toInt()).setEllipsize(TextUtils.TruncateAt.END)
            val l = b.build()
            var w = 0f
            for (i in 0 until l.lineCount) w = max(w, l.getLineWidth(i))
            return Render.encode(Render.TextSize(w = ceil(w) / d, h = l.height / d))
        }

        private val families = HashMap<String, Typeface?>()

        /** The typeface for a font: a family the app bundles (assets/fonts/<family>.ttf or
         *  .otf, added by `nimbus native font add`) or the system's, at the weight. */
        fun fontFor(f: Render.Font, c: Context): Typeface {
            val w = if (f.weight == 0) 400 else f.weight
            val base = if (f.family.isEmpty()) Typeface.DEFAULT else synchronized(families) {
                families.getOrPut(f.family) {
                    listOf("ttf", "otf").firstNotNullOfOrNull { ext ->
                        try {
                            Typeface.createFromAsset(c.assets, "fonts/${f.family}.$ext")
                        } catch (e: RuntimeException) {
                            null
                        }
                    }
                }
            } ?: Typeface.DEFAULT
            if (Build.VERSION.SDK_INT >= 28) return Typeface.create(base, w, false)
            return Typeface.create(base, if (w >= 600) Typeface.BOLD else Typeface.NORMAL)
        }

        /** A rich text (ui.Rich): each run in its own typeface, size, spacing and color. */
        internal fun rich(spans: List<Render.Span>, d: Float, c: Context): CharSequence {
            val out = android.text.SpannableStringBuilder()
            for (sp in spans) {
                val start = out.length
                out.append(sp.text)
                out.setSpan(FontSpan(fontFor(sp.font, c), sp.font, d), start, out.length, android.text.Spanned.SPAN_EXCLUSIVE_EXCLUSIVE)
                if (sp.color != 0L) {
                    out.setSpan(android.text.style.ForegroundColorSpan(argb(sp.color)), start, out.length, android.text.Spanned.SPAN_EXCLUSIVE_EXCLUSIVE)
                }
            }
            return out
        }

        /** One run's font: typeface, size, letter spacing and digits, for drawing and measuring alike. */
        private class FontSpan(val tf: Typeface, val f: Render.Font, val d: Float) : android.text.style.MetricAffectingSpan() {
            override fun updateMeasureState(p: TextPaint) = apply(p)
            override fun updateDrawState(p: TextPaint) = apply(p)
            private fun apply(p: TextPaint) {
                p.typeface = tf
                p.textSize = f.size * d
                applyExtras(p, f, d)
            }
        }

        /** Letter spacing (points → em) and tabular digits on a paint: what text views and
         *  measurement both use, so they agree. */
        internal fun applyExtras(p: Paint, f: Render.Font, d: Float) {
            p.letterSpacing = if (f.tracking != 0f && f.size > 0f) f.tracking / f.size else 0f
            p.fontFeatureSettings = if (f.tabular) "'tnum'" else null
        }

        /** The spacing added to each line for Style.LineHeight (as TextView.setLineHeight does); 0 for the font's own. */
        internal fun lineAdd(p: Paint, f: Render.Font, d: Float): Float =
            if (f.lineHeight > 0f) f.lineHeight * d - p.getFontMetricsInt(null) else 0f

        /** 0xRRGGBBAA → Android's 0xAARRGGBB. */
        internal fun argb(c: Long): Int {
            val v = c.toInt()
            return (v ushr 8) or (v shl 24)
        }

        /** For a scroll view's content group (content → platform scroller → NNScroll), the NNScroll. */
        fun contentOwner(v: View): View =
            ((v.parent as? FrameLayout)?.parent as? NNScroll) ?: v
    }
}
