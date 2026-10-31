package space.nimbusgo.nativekit

import android.content.pm.ActivityInfo

/** orientation: the activity's requested orientation. */
internal object OrientationPlugin {
    fun handle(call: PluginCall) {
        if (call.method != "set") return Plugins.unknownMethod(call)
        Plugins.activity?.requestedOrientation = when (call.string("mode")) {
            "portrait" -> ActivityInfo.SCREEN_ORIENTATION_SENSOR_PORTRAIT
            "landscape" -> ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
            else -> ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
        }
        call.resolve()
    }
}
