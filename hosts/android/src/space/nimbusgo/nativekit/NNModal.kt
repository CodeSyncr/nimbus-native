package space.nimbusgo.nativekit

import android.content.Context

/**
 * A modal: a full-screen layer over the app holding a Go-rendered tree. It slides up when
 * presented and down when dismissed; the back button pops its stack, then closes it.
 */
internal class NNModal(c: Context, r: Renderer, private val modalTag: Int) : NNGroup(c, r) {
    private var reportedW = 0
    private var reportedH = 0

    /** True while sliding away after Go dismissed it: it stays until the animation ends. */
    var leaving = false
        private set

    init {
        setBackgroundColor(0xffffffff.toInt())
        elevation = 16 * c.resources.displayMetrics.density
        clipChildren = true
    }

    fun slideIn() {
        post {
            translationY = height.toFloat()
            animate().translationY(0f).setDuration(SLIDE_MS).start()
        }
    }

    fun slideOut(then: () -> Unit) {
        leaving = true
        animate().translationY(height.toFloat()).setDuration(SLIDE_MS).withEndAction(then).start()
    }

    override fun onSizeChanged(w: Int, h: Int, oldw: Int, oldh: Int) {
        super.onSizeChanged(w, h, oldw, oldh)
        if (w > 0 && (w != reportedW || h != reportedH)) {
            reportedW = w
            reportedH = h
            post { r.resized(modalTag, w, h) }
        }
    }

    private companion object {
        const val SLIDE_MS = 280L
    }
}
