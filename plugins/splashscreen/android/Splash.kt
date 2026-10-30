package space.nimbusgo.nativekit

import android.os.Handler
import android.os.Looper
import android.view.View
import android.view.ViewGroup

/** splashscreen: a view of the launch color over the app until Hide. */
internal object SplashPlugin {
    private var cover: View? = null

    fun boot() {
        val a = Plugins.activity ?: return
        val v = View(a)
        val id = a.resources.getIdentifier("launch_background", "color", a.packageName)
        v.setBackgroundColor(if (id != 0) a.getColor(id) else 0xffffffff.toInt())
        a.addContentView(v, ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT))
        cover = v
        Handler(Looper.getMainLooper()).postDelayed({ hide() }, 10_000)
    }

    private fun hide() {
        val v = cover ?: return
        cover = null
        v.animate().alpha(0f).setDuration(250).withEndAction { (v.parent as? ViewGroup)?.removeView(v) }
    }

    fun handle(call: PluginCall) {
        if (call.method != "hide") return Plugins.unknownMethod(call)
        hide()
        call.resolve()
    }
}
