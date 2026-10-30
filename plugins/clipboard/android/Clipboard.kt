package space.nimbusgo.nativekit

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context

/** clipboard: text on the clipboard. */
internal object ClipboardPlugin {
    fun handle(call: PluginCall) {
        val cm = Plugins.context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        when (call.method) {
            "set" -> {
                cm.setPrimaryClip(ClipData.newPlainText("text", call.string("text") ?: ""))
                call.resolve()
            }
            "get" -> call.resolve(mapOf("text" to (cm.primaryClip?.takeIf { it.itemCount > 0 }?.getItemAt(0)?.coerceToText(Plugins.context)?.toString() ?: "")))
            else -> Plugins.unknownMethod(call)
        }
    }
}
