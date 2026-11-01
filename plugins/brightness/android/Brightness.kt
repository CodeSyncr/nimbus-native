package space.nimbusgo.nativekit

import android.provider.Settings
import android.view.WindowManager

/** brightness: the window's own level, over the system's. */
internal object BrightnessPlugin {
    fun handle(call: PluginCall) {
        val w = Plugins.activity?.window ?: return call.reject(Plugins.FAILED, "no window to change")
        when (call.method) {
            "get" -> {
                val own = w.attributes.screenBrightness
                val level = if (own >= 0) own.toDouble()
                else Settings.System.getInt(Plugins.context.contentResolver, Settings.System.SCREEN_BRIGHTNESS, 128) / 255.0
                call.resolve(mapOf("level" to level))
            }
            "set" -> apply(call, call.args.optDouble("level", 1.0).toFloat())
            "reset" -> apply(call, WindowManager.LayoutParams.BRIGHTNESS_OVERRIDE_NONE)
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun apply(call: PluginCall, level: Float) {
        val w = Plugins.activity?.window ?: return call.reject(Plugins.FAILED, "no window to change")
        w.attributes = w.attributes.apply { screenBrightness = level }
        call.resolve()
    }
}
