package space.nimbusgo.nativekit

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.BlendMode
import android.graphics.Canvas
import android.graphics.DashPathEffect
import android.graphics.LinearGradient
import android.graphics.Matrix
import android.graphics.Paint
import android.graphics.Path
import android.graphics.PorterDuff
import android.graphics.PorterDuffXfermode
import android.graphics.RadialGradient
import android.graphics.RectF
import android.graphics.Region
import android.graphics.Shader
import android.graphics.SweepGradient
import android.os.Build
import android.text.Layout
import android.text.StaticLayout
import android.text.TextPaint
import android.text.TextUtils
import android.util.Log
import android.view.MotionEvent
import android.view.View
import java.net.HttpURLConnection
import java.net.URL
import kotlin.math.abs
import kotlin.math.atan2
import kotlin.math.ceil
import kotlin.math.cos
import kotlin.math.exp
import kotlin.math.hypot
import kotlin.math.ln
import kotlin.math.max
import kotlin.math.min
import kotlin.math.pow
import kotlin.math.sin
import kotlin.math.sqrt

/** A canvas (ui.Canvas): Go's shapes, drawn in a W×H space fitted to the view, with the animations it runs itself. */
internal class NNCanvas(c: android.content.Context, private val tag: Int) : View(c) {
    private var d = Render.Draw()

    /** One of Draw.Anims, and where it is. */
    private class Anim(val a: List<Float>, i: Int) {
        val id = a[i]; val run = a[i + 1]; val kind = a[i + 2].toInt(); val ms = a[i + 3].toDouble(); val delay = a[i + 4].toDouble()
        val ease = a[i + 5].toInt(); val bounce = a[i + 6].toDouble(); val repeats = a[i + 7].toInt(); val rev = a[i + 8].toInt()
        val reverse = rev != 0
        val to = a[i + 9].toDouble(); val lo = a[i + 10].toDouble(); val hi = a[i + 11].toDouble()
    }
    private class Clock(var run: Float, var start: Double, var from: Double, var to: Double, var done: Boolean) {
        var live: Double? = null // where a finger has a drag
    }

    /** A tagged shape as last drawn, in the view's pixels, for taps: topmost last. */
    private class Hit(val n: Int, val path: Path)
    private var hits = ArrayList<Hit>()
    private var ox = 0f
    private var oy = 0f
    private var scale = 1f
    private var anims: List<Anim> = emptyList()
    private val clocks = HashMap<Float, Clock>()

    private fun now() = System.nanoTime() / 1e6

    fun set(o: Render.Draw) {
        val now = now()
        val next = ArrayList<Anim>()
        var i = 0
        while (i + 12 <= o.anims.size) {
            val a = Anim(o.anims, i)
            val c = clocks[a.id]
            if (c == null || (a.kind == 0 && c.run != a.run)) {
                clocks[a.id] = Clock(a.run, now, a.to, a.to, a.kind != 0)
            } else if (a.kind != 0 && c.to != a.to && c.live == null) {
                // A tween's new target: from where it is now.
                c.from = value(anims.firstOrNull { it.id == a.id } ?: a, c, now)
                c.to = a.to
                c.start = now
                c.done = false
            }
            next.add(a)
            i += 12
        }
        val ids = next.map { it.id }.toSet()
        clocks.keys.retainAll(ids)
        anims = next
        d = o
        invalidate()
    }

    /** An animation's number now; it marks the clock done after its last run. */
    private fun value(a: Anim, c: Clock, now: Double): Double {
        val t = now - c.start - a.delay
        if (a.kind != 0) { // a tween, or a drag let go
            c.live?.let { return it }
            if (t <= 0) return c.from
            val (v, len) = eased(a.ease, a.bounce, t, a.ms)
            if (t >= len) { c.done = true; return c.to }
            return c.from + (c.to - c.from) * v
        }
        if (t < 0) return 0.0
        val len = eased(a.ease, a.bounce, 0.0, a.ms).second
        var i = (t / len).toInt()
        val v: Double
        if (a.repeats >= 0 && i > a.repeats) {
            c.done = true
            i = a.repeats
            v = 1.0
        } else {
            v = eased(a.ease, a.bounce, t - i * len, a.ms).first
        }
        return if (a.reverse && i % 2 == 1) 1 - v else v
    }

    // What each Save remembers that the Canvas doesn't: the paint's settings.
    private class State(
        var blend: Int = 0, var shadow: FloatArray? = null, var shadowColor: Int = 0,
        var dash: FloatArray? = null, var cap: Paint.Cap = Paint.Cap.ROUND, var join: Paint.Join = Paint.Join.ROUND,
    ) {
        fun copy() = State(blend, shadow, shadowColor, dash, cap, join)
    }

    private val paint = Paint(Paint.ANTI_ALIAS_FLAG)
    private val text = TextPaint(Paint.ANTI_ALIAS_FLAG)

    override fun onDraw(canvas: Canvas) {
        if (d.w <= 0f || d.h <= 0f) return
        val now = now()
        val vals = DoubleArray(anims.size)
        for ((j, a) in anims.withIndex()) {
            val c = clocks[a.id] ?: continue
            val was = c.done
            vals[j] = value(a, c, now)
            if (c.done && !was && a.kind == 0) {
                Bridge.send(Render.encode(Render.NativeEvent(tag = tag.toLong(), name = "anim", data = "${a.id.toInt()}:${a.run.toInt()}")))
            }
        }
        if (clocks.values.any { !it.done } && isAttachedToWindow) postInvalidateOnAnimation()

        val s = min(width / d.w, height / d.h)
        scale = s
        ox = (width - d.w * s) / 2
        oy = (height - d.h * s) / 2
        val drawn = ArrayList<Hit>()
        val toView = Matrix().also { Matrix(canvas.matrix).invert(it) } // the canvas's matrix back to the view's pixels
        val used = HashMap<String, Int>() // how many times each shader is used in this drawing
        var tagNext = 0
        val base = canvas.save()
        canvas.translate((width - d.w * s) / 2, (height - d.h * s) / 2)
        canvas.scale(s, s)
        var n = 0
        var k = 0
        var si = 0
        var argBase = 0
        var pending: HashMap<Int, Triple<Int, Float, Float>>? = null
        var binds: HashMap<Int, Triple<Int, Float, Float>>? = null
        fun num(): Float {
            val j = n++
            binds?.get(j - argBase)?.let { (a, from, to) -> if (a < vals.size) return (from + (to - from) * vals[a]).toFloat() }
            return if (j < d.nums.size) d.nums[j] else 0f
        }
        fun col(): Int = if (k < d.colors.size) Renderer.argb(d.colors[k++]) else { k++; 0 }
        fun str(): String = if (si < d.strs.size) d.strs[si++] else { si++; "" }
        var path = Path()
        var st = State()
        val stack = ArrayList<Pair<Int, State>>() // the canvas's save count, and the paint's settings
        var trim: FloatArray? = null

        fun fresh() { path = Path() }
        /** Remembers the path as drawn if it's tagged: its outline when stroked w wide. */
        fun hit(w: Float) {
            if (tagNext == 0) return
            val shape = Path()
            if (w > 0) {
                val p = Paint().apply { style = Paint.Style.STROKE; strokeWidth = w; strokeCap = Paint.Cap.ROUND; strokeJoin = Paint.Join.ROUND }
                @Suppress("DEPRECATION") p.getFillPath(path, shape)
            } else {
                shape.set(path)
            }
            shape.transform(Matrix().apply { setConcat(toView, canvas.matrix) })
            drawn.add(Hit(tagNext, shape))
            tagNext = 0
        }
        /** The paint as the state says, for a fill or stroke; false when a trim leaves nothing to draw. */
        fun ready(style: Paint.Style): Boolean {
            paint.reset()
            paint.isAntiAlias = true
            paint.style = style
            paint.strokeCap = st.cap
            paint.strokeJoin = st.join
            blend(paint, st.blend)
            st.shadow?.let { sh ->
                if (st.shadowColor != 0) {
                    // Go's shadow ignores the canvas's transform, as on iOS and the web:
                    // its offset and blur are carried back through the inverse here.
                    val m = Matrix(canvas.matrix)
                    val inv = Matrix()
                    if (m.invert(inv)) {
                        val v = floatArrayOf(sh[0] * s, sh[1] * s)
                        inv.mapVectors(v)
                        val mv = FloatArray(9).also { m.getValues(it) }
                        val scale = sqrt(abs(mv[0] * mv[4] - mv[1] * mv[3])).coerceAtLeast(0.0001f)
                        // CSS's blur is twice the Gaussian's sigma; Android's radius is (sigma - 0.5) / 0.57735.
                        val radius = ((sh[2] * s / 2 - 0.5f) / 0.57735f).coerceAtLeast(0.01f) / scale
                        paint.setShadowLayer(radius, v[0], v[1], st.shadowColor)
                    }
                }
            }
            val t = trim
            if (style == Paint.Style.STROKE && t != null) {
                trim = null
                val vis = (min(t[2], 1f) - max(t[1], 0f)).coerceAtLeast(0f) * t[0]
                if (vis <= 0) return false
                paint.pathEffect = DashPathEffect(floatArrayOf(vis, 2 * t[0]), vis + 2 * t[0] - max(t[1], 0f) * t[0])
            } else if (style == Paint.Style.STROKE) {
                st.dash?.let { if (it.size >= 2) paint.pathEffect = DashPathEffect(if (it.size % 2 == 0) it else it + it, 0f) }
            }
            return true
        }
        fun gradient(): Shader? {
            val kind = num().toInt()
            val x0 = num(); val y0 = num(); val x1 = num(); val y1 = num()
            val count = num().toInt().coerceAtLeast(0)
            val stops = FloatArray(count) { num() }
            val cols = IntArray(count) { col() }
            if (count == 0) return null
            if (count == 1) return LinearGradient(x0, y0, x1, y1, intArrayOf(cols[0], cols[0]), null, Shader.TileMode.CLAMP)
            return when (kind) {
                1 -> RadialGradient(x0, y0, hypot(x1 - x0, y1 - y0).coerceAtLeast(0.01f), cols, stops, Shader.TileMode.CLAMP)
                2 -> SweepGradient(x0, y0, cols, stops).apply {
                    setLocalMatrix(Matrix().apply { setRotate(Math.toDegrees(atan2((y1 - y0).toDouble(), (x1 - x0).toDouble())).toFloat(), x0, y0) })
                }
                else -> LinearGradient(x0, y0, x1, y1, cols, stops, Shader.TileMode.CLAMP)
            }
        }
        loop@ for (b in d.cmds) {
            val cmd = b.toInt()
            if (cmd == 28) { // DrawBind: for the next command's numbers
                val arg = if (n < d.nums.size) d.nums[n].toInt() else 0
                val anim = if (n + 1 < d.nums.size) d.nums[n + 1].toInt() else 0
                val from = if (n + 2 < d.nums.size) d.nums[n + 2] else 0f
                val to = if (n + 3 < d.nums.size) d.nums[n + 3] else 0f
                n += 4
                (pending ?: HashMap<Int, Triple<Int, Float, Float>>().also { pending = it })[arg] = Triple(anim, from, to)
                continue
            }
            binds = pending
            pending = null
            argBase = n
            when (cmd) {
                0 -> path.moveTo(num(), num())
                1 -> path.lineTo(num(), num())
                2 -> path.quadTo(num(), num(), num(), num())
                3 -> path.cubicTo(num(), num(), num(), num(), num(), num())
                4 -> path.close()
                5 -> path.addCircle(num(), num(), num(), Path.Direction.CW)
                6 -> {
                    val x = num(); val y = num(); val w = num(); val h = num()
                    val r = min(num(), min(w, h) / 2).coerceAtLeast(0f)
                    path.addRoundRect(RectF(x, y, x + w, y + h), r, r, Path.Direction.CW)
                }
                7 -> {
                    val c = col()
                    ready(Paint.Style.FILL)
                    paint.color = c
                    canvas.drawPath(path, paint)
                    hit(0f)
                    fresh()
                }
                8 -> {
                    val w = num(); val c = col()
                    if (ready(Paint.Style.STROKE)) {
                        paint.strokeWidth = w
                        paint.color = c
                        canvas.drawPath(path, paint)
                    }
                    hit(w)
                    fresh()
                }
                9 -> {
                    val sh = gradient()
                    ready(Paint.Style.FILL)
                    paint.shader = sh
                    if (sh != null) canvas.drawPath(path, paint)
                    hit(0f)
                    fresh()
                }
                10 -> {
                    val cx = num(); val cy = num(); val r = num(); val start = num(); val sweep = num()
                    val oval = RectF(cx - r, cy - r, cx + r, cy + r)
                    // arcTo takes a sweep mod 360: a whole circle goes in two halves.
                    if (abs(sweep) >= 359.99f) {
                        path.arcTo(oval, start, sweep / 2, true)
                        path.arcTo(oval, start + sweep / 2, sweep / 2, false)
                    } else {
                        path.arcTo(oval, start, sweep, true)
                    }
                }
                11 -> { val x = num(); val y = num(); val w = num(); val h = num(); path.addOval(RectF(x, y, x + w, y + h), Path.Direction.CW) }
                12 -> path.fillType = Path.FillType.EVEN_ODD
                13 -> {
                    val w = num(); val sh = gradient()
                    if (ready(Paint.Style.STROKE) && sh != null) {
                        paint.strokeWidth = w
                        paint.shader = sh
                        canvas.drawPath(path, paint)
                    }
                    hit(w)
                    fresh()
                }
                14 -> { canvas.clipPath(path); fresh() }
                15 -> { stack.add(Pair(canvas.save(), st)); st = st.copy() }
                16 -> {
                    val (count, old) = stack.removeLastOrNull() ?: continue@loop
                    canvas.restoreToCount(count)
                    st = old
                }
                17 -> {
                    val op = num().coerceIn(0f, 1f)
                    // Blended in with the blend mode, without the shadow; the drawing inside keeps it.
                    val p = Paint().apply { alpha = (op * 255).toInt(); blend(this, st.blend) }
                    stack.add(Pair(canvas.saveLayer(null, p), st))
                    st = st.copy().apply { blend = 0 }
                }
                18 -> canvas.translate(num(), num())
                19 -> canvas.rotate(num())
                20 -> canvas.scale(num(), num())
                21 -> st.blend = num().toInt()
                22 -> { st.shadow = floatArrayOf(num(), num(), num()); st.shadowColor = col() }
                23 -> { val m = num().toInt().coerceAtLeast(0); val ls = FloatArray(m) { num() }; st.dash = if (m == 0) null else ls }
                24 -> {
                    st.cap = when (num().toInt()) { 1 -> Paint.Cap.BUTT; 2 -> Paint.Cap.SQUARE; else -> Paint.Cap.ROUND }
                    st.join = when (num().toInt()) { 1 -> Paint.Join.MITER; 2 -> Paint.Join.BEVEL; else -> Paint.Join.ROUND }
                }
                25 -> trim = floatArrayOf(num(), num(), num())
                26 -> {
                    val url = str()
                    val x = num(); val y = num(); val w = num(); val h = num(); val fit = num().toInt()
                    val bmp = picture(url) ?: continue@loop
                    val iw = bmp.width.toFloat(); val ih = bmp.height.toFloat()
                    ready(Paint.Style.FILL)
                    paint.isFilterBitmap = true
                    if (fit == 2 || iw <= 0 || ih <= 0) {
                        canvas.drawBitmap(bmp, null, RectF(x, y, x + w, y + h), paint)
                    } else {
                        val sc = if (fit == 1) min(w / iw, h / ih) else max(w / iw, h / ih)
                        val dw = iw * sc; val dh = ih * sc
                        canvas.save()
                        canvas.clipRect(x, y, x + w, y + h)
                        canvas.drawBitmap(bmp, null, RectF(x + (w - dw) / 2, y + (h - dh) / 2, x + (w + dw) / 2, y + (h + dh) / 2), paint)
                        canvas.restore()
                    }
                }
                27 -> {
                    val s0 = str(); val family = str()
                    val x = num(); val y = num(); val w = num(); val size = num(); val weight = num().toInt()
                    val align = num().toInt(); val lines = num().toInt()
                    val c = col()
                    text.reset()
                    text.isAntiAlias = true
                    text.textSize = size
                    text.typeface = Renderer.fontFor(Render.Font(size = size, weight = weight, family = family), context)
                    text.color = if (c != 0) c else defaultText()
                    val width = if (w > 0) w else ceil(Layout.getDesiredWidth(s0, text)).coerceAtLeast(1f)
                    val l = StaticLayout.Builder.obtain(s0, 0, s0.length, text, ceil(width).toInt().coerceAtLeast(1))
                        .setAlignment(when (align) { 1 -> Layout.Alignment.ALIGN_CENTER; 2 -> Layout.Alignment.ALIGN_OPPOSITE; else -> Layout.Alignment.ALIGN_NORMAL })
                        .setIncludePad(true)
                        .apply { if (lines > 0) setMaxLines(lines).setEllipsize(TextUtils.TruncateAt.END) }
                        .build()
                    canvas.save()
                    canvas.translate(x, y)
                    l.draw(canvas)
                    canvas.restore()
                }
                29 -> tagNext = num().toInt()
                30 -> {
                    num(); num(); num(); num() // the box: Android runs the shader over the path itself
                    val n = num().toInt().coerceAtLeast(0)
                    val u = FloatArray(n) { num() }
                    val fallback = col()
                    val src = str()
                    ready(Paint.Style.FILL)
                    val sh = if (Build.VERSION.SDK_INT >= 33) runtimeShader(src, used.merge(src, 1, Int::plus)!!) else null
                    if (sh != null) {
                        if (n > 0) setUniforms(sh, u)
                        paint.shader = sh
                    } else {
                        paint.color = fallback
                    }
                    canvas.drawPath(path, paint)
                    hit(0f)
                    fresh()
                }
                else -> break@loop // a newer command: stop rather than draw it wrong
            }
        }
        canvas.restoreToCount(base)
        hits = drawn
    }

    /** The topmost tagged shape at (x, y), 0 for none. */
    private fun shape(x: Float, y: Float): Int {
        for (h in hits.asReversed()) {
            val b = RectF().also { h.path.computeBounds(it, true) }
            if (!b.contains(x, y)) continue
            val r = Region().apply { setPath(h.path, Region(b.left.toInt() - 1, b.top.toInt() - 1, b.right.toInt() + 1, b.bottom.toInt() + 1)) }
            if (r.contains(x.toInt(), y.toInt())) return h.n
        }
        return 0
    }

    private class Down(val x: Float, val y: Float, val shape: Int, val drag: Anim?, val v0: Double) {
        var moved = false
        var dragging = false
    }
    private var down: Down? = null
    private val slop = android.view.ViewConfiguration.get(c).scaledTouchSlop

    /** Taps and drags (Draw.Touch): a drag moves its number here, frame by frame, and Go hears where it was let go. */
    override fun onTouchEvent(e: MotionEvent): Boolean {
        if (!d.touch) return super.onTouchEvent(e)
        when (e.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                val n = shape(e.x, e.y)
                val a = anims.firstOrNull { it.kind == 2 && n != 0 && it.rev == n } ?: anims.firstOrNull { it.kind == 2 && it.rev == 0 }
                val v0 = a?.let { clocks[it.id]?.let { c -> value(it, c, now()) } } ?: 0.0
                down = Down(e.x, e.y, n, a, v0)
            }
            MotionEvent.ACTION_MOVE -> {
                val dn = down ?: return true
                val dx = e.x - dn.x
                val dy = e.y - dn.y
                if (!dn.moved && hypot(dx, dy) > slop) {
                    dn.moved = true
                    // Along the drag's axis it's ours; across it, the scroll around us takes it.
                    val a = dn.drag
                    if (a != null && (if (a.repeats == 1) abs(dy) >= abs(dx) else abs(dx) >= abs(dy))) {
                        dn.dragging = true
                        parent?.requestDisallowInterceptTouchEvent(true)
                    }
                }
                val a = dn.drag
                if (dn.dragging && a != null) {
                    clocks[a.id]?.live = min(max(dn.v0 + (if (a.repeats == 1) dy else dx) / scale, a.lo), a.hi)
                    invalidate()
                }
            }
            MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                val dn = down ?: return true
                down = null
                val a = dn.drag
                if (dn.dragging && a != null) {
                    val c = clocks[a.id]
                    val v = c?.live
                    if (c != null && v != null) {
                        c.live = null; c.from = v; c.to = v; c.done = true
                        send("drag", "${a.id.toInt()}:$v")
                        invalidate()
                    }
                } else if (!dn.moved && e.actionMasked == MotionEvent.ACTION_UP) {
                    performClick()
                    send("tap", "${dn.shape},${(dn.x - ox) / scale},${(dn.y - oy) / scale}")
                }
            }
        }
        return true
    }

    override fun performClick(): Boolean = super.performClick()

    private fun send(name: String, data: String) =
        Bridge.send(Render.encode(Render.NativeEvent(tag = tag.toLong(), name = name, data = data)))

    // Each use of a shader in a drawing has its own instance, so one use's
    // uniforms don't change another's; compiled once, kept for the next frame.
    private val shaders = HashMap<String, Any?>()

    private fun runtimeShader(src: String, use: Int): Shader? {
        if (Build.VERSION.SDK_INT < 33) return null
        val key = "$use\u0000$src"
        if (!shaders.containsKey(key)) {
            shaders[key] = try {
                android.graphics.RuntimeShader(src)
            } catch (e: IllegalArgumentException) {
                Log.w("NimbusNative", "shader: ${e.message}")
                null
            }
        }
        return shaders[key] as Shader?
    }

    private fun setUniforms(sh: Shader, u: FloatArray) {
        if (Build.VERSION.SDK_INT >= 33) (sh as android.graphics.RuntimeShader).setFloatUniform("nn_u", u)
    }

    private fun defaultText(): Int {
        val a = context.obtainStyledAttributes(intArrayOf(android.R.attr.textColorPrimary))
        try {
            return a.getColor(0, 0xff111111.toInt())
        } finally {
            a.recycle()
        }
    }

    /** The picture at url if it's loaded; otherwise null, and a redraw once it is. */
    private fun picture(url: String): Bitmap? {
        Renderer.bitmaps.get(url)?.let { return it }
        synchronized(loading) { if (!loading.add(url)) return null }
        Renderer.loader.execute {
            Renderer.diskCache(context)
            val b = try {
                val conn = URL(url).openConnection() as HttpURLConnection
                conn.instanceFollowRedirects = true
                conn.inputStream.use { BitmapFactory.decodeStream(it) }
            } catch (e: Exception) {
                Log.w("NimbusNative", "canvas image $url: $e")
                null
            }
            synchronized(loading) { loading.remove(url) }
            if (b != null) {
                Renderer.bitmaps.put(url, b)
                Renderer.main.post { invalidate() }
            }
        }
        return null
    }

    companion object {
        private val loading = HashSet<String>()

        /** eased is ui/animate.go's: where a run is t ms in, and how long a run takes. */
        fun eased(e: Int, bounce: Double, t: Double, ms: Double): Pair<Double, Double> {
            if (e == 4) {
                val z = min(max(1 - bounce, 0.05), 1.0)
                val w = 2 * Math.PI
                if (z == 1.0) {
                    val len = ms * 9.24 / w
                    val u = min(t, len) / ms
                    return Pair(1 - exp(-w * u) * (1 + w * u), len)
                }
                val wd = w * sqrt(1 - z * z)
                val len = ms * max(1.0, ln(1000 * (1 + z * w / wd)) / (z * w))
                val u = min(t, len) / ms
                return Pair(1 - exp(-z * w * u) * (cos(wd * u) + z * w / wd * sin(wd * u)), len)
            }
            val x = min(max(t / ms, 0.0), 1.0)
            return Pair(when (e) {
                1 -> x
                2 -> x * x * x
                3 -> 1 - (1 - x).pow(3)
                else -> if (x < 0.5) 4 * x * x * x else 1 - 4 * (1 - x).pow(3)
            }, ms)
        }

        private val blends29 = arrayOf(
            BlendMode.SRC_OVER, BlendMode.MULTIPLY, BlendMode.SCREEN, BlendMode.OVERLAY, BlendMode.DARKEN, BlendMode.LIGHTEN,
            BlendMode.COLOR_DODGE, BlendMode.COLOR_BURN, BlendMode.HARD_LIGHT, BlendMode.SOFT_LIGHT, BlendMode.DIFFERENCE,
            BlendMode.EXCLUSION, BlendMode.HUE, BlendMode.SATURATION, BlendMode.COLOR, BlendMode.LUMINOSITY,
            BlendMode.DST_OUT, BlendMode.SRC_ATOP, BlendMode.PLUS,
        )

        /** Sets a blend mode: Android 10's BlendMode, or the Porter-Duff modes older ones have. */
        fun blend(p: Paint, mode: Int) {
            if (mode <= 0) return
            if (Build.VERSION.SDK_INT >= 29) {
                p.blendMode = blends29.getOrNull(mode)
                return
            }
            val pd = when (mode) {
                1 -> PorterDuff.Mode.MULTIPLY; 2 -> PorterDuff.Mode.SCREEN; 3 -> PorterDuff.Mode.OVERLAY
                4 -> PorterDuff.Mode.DARKEN; 5 -> PorterDuff.Mode.LIGHTEN; 16 -> PorterDuff.Mode.DST_OUT
                17 -> PorterDuff.Mode.SRC_ATOP; 18 -> PorterDuff.Mode.ADD
                else -> null
            }
            p.xfermode = pd?.let { PorterDuffXfermode(it) }
        }
    }
}
