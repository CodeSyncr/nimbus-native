package space.nimbusgo.nativekit

import android.content.Context
import android.view.accessibility.AccessibilityManager

/** screenreader: TalkBack. */
internal object ScreenReaderPlugin {
    fun handle(call: PluginCall) {
        val am = Plugins.context.getSystemService(Context.ACCESSIBILITY_SERVICE) as AccessibilityManager
        when (call.method) {
            "enabled" -> call.resolve(mapOf("enabled" to (am.isEnabled && am.isTouchExplorationEnabled)))
            "announce" -> {
                @Suppress("DEPRECATION")
                Plugins.activity?.window?.decorView?.announceForAccessibility(call.string("text") ?: "")
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
