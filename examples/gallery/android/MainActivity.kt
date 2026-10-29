package space.nimbusgo.gallery

import android.app.Activity
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.Log
import android.view.Choreographer
import android.view.View
import android.view.ViewGroup
import android.widget.EditText
import android.widget.TextView
import space.nimbusgo.nativekit.NimbusView
import space.nimbusgo.nativekit.Render
import space.nimbusgo.nativekit.Renderer

/**
 * The gallery's Android host: one NimbusView, plus a benchmark that runs by itself after launch
 * and logs to logcat (tag NimbusNativeApp).
 */
class MainActivity : Activity() {
    private lateinit var r: Renderer
    private val main = Handler(Looper.getMainLooper())
    private var commits = 0
    private var pressAt = 0L
    private val taps = mutableListOf<Double>()
    private val frameTimes = mutableListOf<Long>()
    private var typing = false
    private var scrolling = false
    private var pushing = false
    private var listOK = false
    private var scrollCommits = 0

    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        val nimbus = NimbusView(this, NimbusSchema.COMBINED)
        setContentView(nimbus)
        r = nimbus.renderer
        r.onCommit = Renderer.CommitListener(::committed)
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        @Suppress("DEPRECATION")
        if (!r.back()) super.onBackPressed() // stacks first, then leave the app
    }

    private fun committed(c: Render.Commit, applyMs: Double) {
        commits++
        if (scrolling) scrollCommits++
        if (pushing) {
            pushing = false
            log("push: row tap → detail screen committed in %.2f ms (%d ops; Go build %.2f ms, layout %.2f ms with %d text measurements; apply incl. push %.2f ms)".format(
                (SystemClock.elapsedRealtimeNanos() - pressAt) / 1e6, c.ops.size, c.buildNs / 1e6, c.layoutNs / 1e6, c.measures, applyMs))
            pressAt = 0
            return
        }
        if (typing) {
            typing = false
            val greeting = (r.viewForTag(r.tagForTestID("greeting")) as? TextView)?.text?.toString() ?: "?"
            log("typed \"Yash\" → greeting: $greeting")
            if (greeting != "Hello, Yash 👋") return log("FAIL: greeting did not follow the input")
            main.post(::scroll)
            return
        }
        if (commits == 1) {
            val views = c.ops.count { it is Render.Create }
            log("first render: %d native views, %d ops; Go build %.1f ms, layout %.1f ms (%d text measurements); apply %.1f ms".format(
                views, c.ops.size, c.buildNs / 1e6, c.layoutNs / 1e6, c.measures, applyMs))
            main.postDelayed(::tap, 1500)
            return
        }
        if (pressAt == 0L) return
        // Count to the end of the frame that shows the change, not just the apply.
        val pressed = pressAt
        pressAt = 0
        Choreographer.getInstance().postFrameCallback {
            main.post {
                taps.add((SystemClock.elapsedRealtimeNanos() - pressed) / 1e6)
                if (taps.size == 1) {
                    log("tap commit: %d ops; Go build %.2f ms, layout %.2f ms; apply %.2f ms".format(
                        c.ops.size, c.buildNs / 1e6, c.layoutNs / 1e6, applyMs))
                }
                if (taps.size < 20) return@post main.postDelayed(::tap, 50).let {}
                val s = taps.sorted()
                log("tap → screen: 20 taps, median %.2f ms, worst %.2f ms".format(s[10], s[19]))
                val counter = r.viewForTag(r.tagForTestID("counter")) as? ViewGroup
                log("counter label now: ${(counter?.getChildAt(0) as? TextView)?.text ?: "?"}")
                typeName()
            }
        }
    }

    private fun tap() {
        val tag = r.tagForTestID("counter")
        if (tag == 0) return log("FAIL: no counter")
        pressAt = SystemClock.elapsedRealtimeNanos()
        r.press(tag)
    }

    /** Types into the name field the way the keyboard does. */
    private fun typeName() {
        val f = r.viewForTag(r.tagForTestID("name")) as? EditText ?: return log("FAIL: no name field")
        typing = true
        f.setText("Yash")
    }

    private fun scroll() {
        val list = r.viewForTag(r.tagForTestID("list")) ?: return log("FAIL: no list")
        val step = (40 * resources.displayMetrics.density).toInt()
        scrolling = true
        Choreographer.getInstance().postFrameCallback(object : Choreographer.FrameCallback {
            override fun doFrame(t: Long) {
                frameTimes.add(t)
                list.scrollBy(0, step)
                if (frameTimes.size < 180) return Choreographer.getInstance().postFrameCallback(this)
                @Suppress("DEPRECATION")
                val target = 1000.0 / windowManager.defaultDisplay.refreshRate
                val dropped = frameTimes.zipWithNext().count { (a, b) -> (b - a) / 1e6 > target * 1.5 }
                val secs = (frameTimes.last() - frameTimes.first()) / 1e9
                log("scroll: 180 frames through the list, %.1f fps, %d dropped (target %.1f ms/frame), %d window commits".format(
                    (frameTimes.size - 1) / secs, dropped, target, scrollCommits))
                scrolling = false
                main.postDelayed({ checkCoverage(list, dropped) }, 300)
            }
        })
    }

    /** Rows (groups with children; spacers have none) must cover the visible area. */
    private fun checkCoverage(list: View, dropped: Int) {
        val content = (list as ViewGroup).getChildAt(0) as ViewGroup
        val top = list.scrollY
        val bottom = top + list.height
        var reach = 0
        var rows = 0
        for (i in 0 until content.childCount) {
            val v = content.getChildAt(i)
            if (v is ViewGroup && v.childCount > 0 && v.bottom > top && v.top < bottom) {
                rows++
                reach = maxOf(reach, v.bottom)
            }
        }
        log("after scroll: %d rows on screen, rows reach %d of %d px; %d native views in the list".format(
            rows, reach - top, bottom - top, content.childCount))
        listOK = taps.sorted()[10] < 16 && dropped < 9 && reach >= bottom
        navigate(content, top, bottom)
    }

    /** Taps the first row on screen, then presses the system back button. */
    private fun navigate(content: ViewGroup, top: Int, bottom: Int) {
        val row = (0 until content.childCount).map(content::getChildAt).firstOrNull {
            (it.tag as? String)?.startsWith("row-") == true && it.bottom > top && it.top < bottom
        } ?: return log("FAIL: no row on screen")
        pushing = true
        pressAt = SystemClock.elapsedRealtimeNanos()
        r.press(row.id)
        main.postDelayed({
            val title = r.viewForTag(r.tagForTestID("detail-title")) as? TextView
            log("after push: detail shows ${title?.text ?: "nothing"}")
            val pushed = title != null
            @Suppress("DEPRECATION")
            onBackPressed() // what the system back button does
            main.postDelayed({
                val goFollowed = r.tagForTestID("detail-title") == 0
                log("after system back: Go dropped the detail screen: $goFollowed")
                log(if (listOK && pushed && goFollowed) "NN-P1 RESULT: PASS" else "NN-P1 RESULT: CHECK (see numbers above)")
            }, 1000)
        }, 1000)
    }

    private fun log(s: String) {
        Log.i("NimbusNativeApp", "[gallery] $s")
    }
}
