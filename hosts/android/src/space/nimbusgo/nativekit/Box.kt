package space.nimbusgo.nativekit

import android.graphics.Canvas
import android.graphics.ColorFilter
import android.graphics.LinearGradient
import android.graphics.Outline
import android.graphics.Paint
import android.graphics.Path
import android.graphics.PixelFormat
import android.graphics.RadialGradient
import android.graphics.RectF
import android.graphics.RenderEffect
import android.graphics.RenderNode
import android.graphics.Shader
import android.graphics.drawable.Drawable
import android.os.Build
import android.view.View
import android.view.ViewTreeObserver
import kotlin.math.hypot
import kotlin.math.min

/**
 * A box's look as one drawable: what is behind it blurred (Android 12+), its background tint,
 * a gradient over that, and its border, all inside its corners (any of them rounded).
 */
internal class BoxDrawable(private val view: View, private val o: Render.Box, private val density: Float) : Drawable() {
    private val fill = Paint(Paint.ANTI_ALIAS_FLAG)
    private val shade = Paint(Paint.ANTI_ALIAS_FLAG)
    private val stroke = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.STROKE }
    private val path = Path()
    private val rect = RectF()
    private val backdrop = if (o.blur != Render.Blur.NONE && Build.VERSION.SDK_INT >= 31) Backdrop(view, blurRadius(o.blur) * density) else null

    /** The radius of each corner (top left, top right, bottom right, bottom left), at most half the short side. */
    fun radii(w: Float, h: Float): FloatArray {
        val r = min(o.radius * density, min(w, h) / 2)
        val all = o.corners == 0
        fun at(bit: Int) = if (all || o.corners and bit != 0) r else 0f
        return floatArrayOf(at(1), at(2), at(4), at(8))
    }

    fun outline(w: Int, h: Int): Path {
        val c = radii(w.toFloat(), h.toFloat())
        rect.set(0f, 0f, w.toFloat(), h.toFloat())
        return Path().apply { addRoundRect(rect, floatArrayOf(c[0], c[0], c[1], c[1], c[2], c[2], c[3], c[3]), Path.Direction.CW) }
    }

    override fun draw(canvas: Canvas) {
        val b = bounds
        if (b.isEmpty) return
        path.reset()
        path.addPath(outline(b.width(), b.height()))
        backdrop?.draw(canvas, path)
        if (o.background != 0L) {
            fill.color = Renderer.argb(o.background)
            canvas.drawPath(path, fill)
        }
        if (o.gradientColors.size >= 2) {
            val w = b.width().toFloat()
            val h = b.height().toFloat()
            val colors = o.gradientColors.map { Renderer.argb(it) }.toIntArray()
            val stops = if (o.gradientStops.size == o.gradientColors.size) o.gradientStops.toFloatArray() else null
            shade.shader = if (o.gradientRadial) {
                val r = hypot((o.gradientX1 - o.gradientX0) * w, (o.gradientY1 - o.gradientY0) * h).coerceAtLeast(1f)
                RadialGradient(o.gradientX0 * w, o.gradientY0 * h, r, colors, stops, Shader.TileMode.CLAMP)
            } else {
                LinearGradient(o.gradientX0 * w, o.gradientY0 * h, o.gradientX1 * w, o.gradientY1 * h, colors, stops, Shader.TileMode.CLAMP)
            }
            canvas.drawPath(path, shade)
        }
        if (o.borderWidth > 0) {
            val sw = maxOf(1f, o.borderWidth * density)
            stroke.strokeWidth = sw
            stroke.color = Renderer.argb(o.borderColor)
            // Inset by half the stroke, so the border sits inside the box as on iOS and the web.
            val inset = Path()
            val c = radii(b.width() - sw, b.height() - sw)
            rect.set(sw / 2, sw / 2, b.width() - sw / 2, b.height() - sw / 2)
            inset.addRoundRect(rect, floatArrayOf(c[0], c[0], c[1], c[1], c[2], c[2], c[3], c[3]), Path.Direction.CW)
            canvas.drawPath(inset, stroke)
        }
    }

    /** The outline follows the corners: for clipping, and for a shadow's shape. */
    override fun getOutline(outline: Outline) {
        val b = bounds
        if (Build.VERSION.SDK_INT >= 30) outline.setPath(outline(b.width(), b.height()))
        else outline.setRoundRect(b, radii(b.width().toFloat(), b.height().toFloat())[0])
    }

    val blurs get() = backdrop != null

    override fun setAlpha(alpha: Int) {}
    override fun setColorFilter(colorFilter: ColorFilter?) {}
    @Deprecated("Deprecated in Java")
    override fun getOpacity() = PixelFormat.TRANSLUCENT

    fun detach() = backdrop?.detach()

    companion object {
        /** Blur radius in dp for each strength, close to iOS's materials. */
        fun blurRadius(b: Int) = when (b) {
            Render.Blur.ULTRA_THIN -> 8f
            Render.Blur.THIN -> 14f
            Render.Blur.REGULAR -> 22f
            else -> 30f
        }
    }
}

/**
 * What is behind a view, blurred (Android 12+). The window is drawn in software into a small
 * bitmap, leaving out the view and all it holds ([capturing]); the bitmap is then scaled up into a
 * render node under a blur effect and drawn through the view's corners. (Recording the window into
 * a render node instead would record this view's own display list, which holds that node: a loop.)
 * It is redrawn whenever the window draws something else.
 */
internal class Backdrop(private val view: View, private val radius: Float) {
    private val node = RenderNode("nimbus-backdrop")
    private val here = IntArray(2)
    private val root = IntArray(2)
    private var bitmap: android.graphics.Bitmap? = null
    private val scaled = Paint(Paint.FILTER_BITMAP_FLAG)
    private var ours = false // the last frame was asked for by us, not by a change elsewhere
    private var captured = false
    // Before each frame the window draws, capture what is behind (outside any draw pass), and
    // draw again with it; a frame we asked for ourselves doesn't capture again, or it never stops.
    private val listener = ViewTreeObserver.OnPreDrawListener {
        if (ours) ours = false else if (view.isShown) { captured = capture(); ours = true; view.invalidate() }
        true
    }
    private val attachment = object : View.OnAttachStateChangeListener {
        override fun onViewAttachedToWindow(v: View) = v.viewTreeObserver.addOnPreDrawListener(listener)
        override fun onViewDetachedFromWindow(v: View) = v.viewTreeObserver.removeOnPreDrawListener(listener)
    }

    init {
        view.addOnAttachStateChangeListener(attachment)
        if (view.isAttachedToWindow) view.viewTreeObserver.addOnPreDrawListener(listener)
    }

    /** Stops following the window (the view's look changed, or it is gone). */
    fun detach() {
        view.removeOnAttachStateChangeListener(attachment)
        view.viewTreeObserver.removeOnPreDrawListener(listener)
    }

    /** Draws the window behind the view into the small bitmap; false if it couldn't. */
    private fun capture(): Boolean {
        val top = view.rootView
        val w = view.width
        val h = view.height
        if (w <= 0 || h <= 0) return false
        val bw = maxOf(1, w / SCALE)
        val bh = maxOf(1, h / SCALE)
        val b = bitmap?.takeIf { it.width == bw && it.height == bh }
            ?: android.graphics.Bitmap.createBitmap(bw, bh, android.graphics.Bitmap.Config.ARGB_8888).also { bitmap = it }
        b.eraseColor(0)
        view.getLocationInWindow(here)
        top.getLocationInWindow(root)
        val sc = Canvas(b)
        sc.scale(1f / SCALE, 1f / SCALE)
        sc.translate(-(here[0] - root[0]).toFloat(), -(here[1] - root[1]).toFloat())
        capturing = true
        try {
            top.draw(sc)
        } catch (_: Throwable) {
            return false // a view that can't be drawn in software: no blur this frame
        } finally {
            capturing = false
        }
        return true
    }

    fun draw(canvas: Canvas, clip: Path) {
        if (Build.VERSION.SDK_INT < 31 || !canvas.isHardwareAccelerated || capturing || !captured) return
        val b = bitmap ?: return
        val w = view.width
        val h = view.height
        node.setPosition(0, 0, w, h)
        val rc = node.beginRecording(w, h)
        rc.scale(SCALE.toFloat(), SCALE.toFloat())
        rc.drawBitmap(b, 0f, 0f, scaled)
        node.endRecording()
        node.setRenderEffect(RenderEffect.createBlurEffect(radius, radius, Shader.TileMode.CLAMP))
        canvas.save()
        canvas.clipPath(clip)
        canvas.drawRenderNode(node)
        canvas.restore()
    }

    companion object {
        /** True while a backdrop draws the window: views with a backdrop draw nothing then. */
        @JvmStatic var capturing = false

        /** The window is drawn at 1/SCALE size: the blur hides the difference. */
        private const val SCALE = 6
    }
}
