package space.nimbusgo.nativekit

import android.view.MotionEvent
import android.view.ScaleGestureDetector
import android.view.VelocityTracker
import android.view.ViewConfiguration
import android.view.ViewGroup
import kotlin.math.abs
import kotlin.math.hypot

private const val PAN_X = 2
private const val PAN_Y = 3

private const val BEGAN = 0
private const val CHANGED = 1
private const val ENDED = 2
private const val CANCELLED = 3

/**
 * A view's drags and pinches. A drag takes the touch from the view's children, and from scrollers
 * around it, once the finger has moved past the slop the way it allows; measured on the screen, so
 * a view that follows the finger doesn't change it.
 */
internal class Gestures(private val v: ViewGroup, private val r: Renderer, private val tag: Int) {
    var pan = 0
    var pinch = false

    private val slop = ViewConfiguration.get(v.context).scaledTouchSlop
    private var startX = 0f
    private var startY = 0f
    private var dragging = false
    private var gaveUp = false
    private var velocity: VelocityTracker? = null

    private var pinching = false
    private var scale = 1f
    private val pinches = ScaleGestureDetector(v.context, object : ScaleGestureDetector.SimpleOnScaleGestureListener() {
        override fun onScaleBegin(d: ScaleGestureDetector): Boolean {
            if (!pinch) return false
            pinching = true
            scale = 1f
            v.parent?.requestDisallowInterceptTouchEvent(true)
            r.pinched(tag, BEGAN, 1f)
            return true
        }

        override fun onScale(d: ScaleGestureDetector): Boolean {
            scale *= d.scaleFactor
            r.pinched(tag, CHANGED, scale)
            return true
        }

        override fun onScaleEnd(d: ScaleGestureDetector) {
            pinching = false
            r.pinched(tag, ENDED, scale)
        }
    })

    /** Whether the view should take this touch from its children. */
    fun intercept(e: MotionEvent): Boolean {
        track(e)
        return dragging || pinching
    }

    /** Whether the view used this touch. */
    fun touch(e: MotionEvent): Boolean {
        track(e)
        return e.actionMasked == MotionEvent.ACTION_DOWN || dragging || pinching
    }

    private var seenAt = -1L
    private var seenAction = -1

    /** A touch that passed through intercept reaches touch too: it counts once. Android reuses
     *  its event objects, so an event is known by its time and action. */
    private fun track(e: MotionEvent) {
        if (e.eventTime == seenAt && e.action == seenAction) return
        seenAt = e.eventTime
        seenAction = e.action
        if (pinch) pinches.onTouchEvent(e)
        when (e.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                startX = e.rawX
                startY = e.rawY
                dragging = false
                gaveUp = false
                velocity?.recycle()
                velocity = VelocityTracker.obtain()
                velocity?.addMovement(moved(e))
            }
            MotionEvent.ACTION_POINTER_DOWN -> if (dragging) finish(e, CANCELLED)
            MotionEvent.ACTION_MOVE -> {
                velocity?.addMovement(moved(e))
                if (pan == 0 || pinching || gaveUp) return
                val dx = e.rawX - startX
                val dy = e.rawY - startY
                if (dragging) return report(CHANGED, dx, dy)
                if (hypot(dx, dy) < slop) return
                val wrongWay = (pan == PAN_X && abs(dy) > abs(dx)) || (pan == PAN_Y && abs(dx) > abs(dy))
                if (wrongWay) {
                    gaveUp = true
                    return
                }
                dragging = true
                v.parent?.requestDisallowInterceptTouchEvent(true)
                v.isPressed = false
                r.panned(tag, BEGAN, dx, dy, 0f, 0f)
            }
            MotionEvent.ACTION_UP -> if (dragging) finish(e, ENDED)
            MotionEvent.ACTION_CANCEL -> if (dragging) finish(e, CANCELLED)
        }
    }

    /** The touch in screen coordinates, for the tracker: the view may be moving under it. */
    private fun moved(e: MotionEvent): MotionEvent = MotionEvent.obtain(e).also { it.setLocation(e.rawX, e.rawY) }

    private fun report(phase: Int, dx: Float, dy: Float) {
        val t = velocity
        t?.computeCurrentVelocity(1000)
        r.panned(tag, phase, dx, dy, t?.xVelocity ?: 0f, t?.yVelocity ?: 0f)
    }

    private fun finish(e: MotionEvent, phase: Int) {
        report(phase, e.rawX - startX, e.rawY - startY)
        dragging = false
        velocity?.recycle()
        velocity = null
    }
}
