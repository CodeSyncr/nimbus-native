package space.nimbusgo.nativekit

import android.app.Activity
import android.os.Build
import android.view.WindowManager

/** screencapture: the window's secure flag, and Android 14's report of screenshots. */
internal object ScreenCapturePlugin {
    private var count = 0
    private var callback: Activity.ScreenCaptureCallback? = null

    fun handle(call: PluginCall) {
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no window to change")
        when (call.method) {
            "set" -> {
                val f = WindowManager.LayoutParams.FLAG_SECURE
                if (call.args.optBoolean("prevent")) a.window.addFlags(f) else a.window.clearFlags(f)
                call.resolve()
            }
            "start" -> {
                if (Build.VERSION.SDK_INT < 34) return call.reject(Plugins.UNAVAILABLE, "Android reports screenshots from version 14")
                count = 0
                val c = Activity.ScreenCaptureCallback { count++ }
                callback = c
                a.registerScreenCaptureCallback(a.mainExecutor, c)
                call.resolve()
            }
            "read" -> call.resolve(mapOf("count" to count))
            "stop" -> {
                if (Build.VERSION.SDK_INT >= 34) callback?.let { a.unregisterScreenCaptureCallback(it) }
                callback = null
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
