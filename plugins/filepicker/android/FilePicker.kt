package space.nimbusgo.nativekit

import android.app.Activity
import android.content.Intent
import android.provider.OpenableColumns
import android.util.Base64
import kotlin.concurrent.thread

/** filepicker: the system's document picker. */
internal object FilePickerPlugin {
    fun handle(call: PluginCall) {
        if (call.method != "pick") return Plugins.unknownMethod(call)
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        val types = call.args.optJSONArray("types")?.let { t -> Array(t.length()) { t.getString(it) } } ?: emptyArray()
        val i = Intent(Intent.ACTION_OPEN_DOCUMENT).addCategory(Intent.CATEGORY_OPENABLE).setType("*/*")
        if (types.isNotEmpty()) i.putExtra(Intent.EXTRA_MIME_TYPES, types)
        ResultAsker.start(a, i) { result, data ->
            val uri = data?.data
            if (result != Activity.RESULT_OK || uri == null) return@start call.reject(Plugins.CANCELLED, "no file")
            val most = call.args.optLong("maxBytes", 0).let { if (it > 0) it else 20L shl 20 }
            thread {
                try {
                    val r = a.contentResolver
                    var name = "file"
                    r.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { if (it.moveToFirst()) name = it.getString(0) }
                    val bytes = r.openInputStream(uri)!!.use { it.readBytes() }
                    if (bytes.size > most) return@thread call.reject(Plugins.FAILED, "the file is too large")
                    call.resolve(mapOf("name" to name, "type" to (r.getType(uri) ?: ""), "data" to Base64.encodeToString(bytes, Base64.NO_WRAP)))
                } catch (e: Exception) {
                    call.reject(Plugins.FAILED, e.toString())
                }
            }
        }
    }
}
