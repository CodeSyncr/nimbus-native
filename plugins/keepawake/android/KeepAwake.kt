package space.nimbusgo.nativekit

import android.view.WindowManager

/** keepawake: a window flag. */
internal object KeepAwakePlugin {
    fun handle(call: PluginCall) {
        if (call.method != "set") return Plugins.unknownMethod(call)
        val w = Plugins.activity?.window ?: return call.resolve()
        val f = WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON
        if (call.args.optBoolean("on")) w.addFlags(f) else w.clearFlags(f)
        call.resolve()
    }
}
