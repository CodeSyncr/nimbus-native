package space.nimbusgo.nativekit

import android.os.Build
import android.view.View
import android.view.WindowInsets
import android.view.WindowInsetsController

/** statusbar: on the activity's window. */
internal object StatusBarPlugin {
    @Suppress("DEPRECATION")
    fun handle(call: PluginCall) {
        if (call.method != "set") return Plugins.unknownMethod(call)
        val w = Plugins.activity?.window ?: return call.resolve()
        val light = View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR
        when (call.string("style")) {
            "dark" -> w.decorView.systemUiVisibility = w.decorView.systemUiVisibility or light
            "light" -> w.decorView.systemUiVisibility = w.decorView.systemUiVisibility and light.inv()
        }
        if (call.args.has("hidden")) {
            val hide = call.args.optBoolean("hidden")
            if (Build.VERSION.SDK_INT >= 30) {
                w.insetsController?.let {
                    if (hide) {
                        it.hide(WindowInsets.Type.statusBars())
                        it.systemBarsBehavior = WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
                    } else {
                        it.show(WindowInsets.Type.statusBars())
                    }
                }
            } else {
                val f = View.SYSTEM_UI_FLAG_FULLSCREEN
                w.decorView.systemUiVisibility = if (hide) w.decorView.systemUiVisibility or f else w.decorView.systemUiVisibility and f.inv()
            }
        }
        if (call.args.has("color")) w.statusBarColor = Renderer.argb(call.args.optLong("color"))
        call.resolve()
    }
}
