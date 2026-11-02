package space.nimbusgo.nativekit

import android.app.Activity
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.OpenableColumns
import android.util.Base64
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.util.UUID

/** The share sheet sends here. It copies what came while it may still read it, then opens the app. */
class ShareReceiverActivity : Activity() {
    override fun onCreate(saved: Bundle?) {
        super.onCreate(saved)
        try {
            intent?.let { ReceiveSharePlugin.receive(this, it) }
        } finally {
            packageManager.getLaunchIntentForPackage(packageName)?.let {
                startActivity(it.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP))
            }
            finish()
        }
    }
}

/** receiveshare: what arrived, kept until Go takes it. Files wait in the cache. */
internal object ReceiveSharePlugin {
    private val inbox = ArrayList<JSONObject>()

    private fun folder(a: Activity) = File(a.cacheDir, "nimbus-shared").apply { mkdirs() }

    @Suppress("DEPRECATION")
    private fun streams(i: Intent): List<Uri> = when (i.action) {
        Intent.ACTION_SEND -> listOfNotNull(
            if (Build.VERSION.SDK_INT >= 33) i.getParcelableExtra(Intent.EXTRA_STREAM, Uri::class.java) else i.getParcelableExtra(Intent.EXTRA_STREAM))
        Intent.ACTION_SEND_MULTIPLE ->
            (if (Build.VERSION.SDK_INT >= 33) i.getParcelableArrayListExtra(Intent.EXTRA_STREAM, Uri::class.java) else i.getParcelableArrayListExtra(Intent.EXTRA_STREAM)) ?: emptyList()
        else -> emptyList()
    }

    fun receive(a: Activity, i: Intent) {
        if (i.action != Intent.ACTION_SEND && i.action != Intent.ACTION_SEND_MULTIPLE) return
        val text = i.getCharSequenceExtra(Intent.EXTRA_TEXT)?.toString() ?: ""
        val files = JSONArray()
        for (u in streams(i)) {
            try {
                val r = a.contentResolver
                var name = u.lastPathSegment ?: "file"
                r.query(u, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { if (it.moveToFirst() && !it.isNull(0)) name = it.getString(0) }
                val to = File(folder(a), UUID.randomUUID().toString())
                r.openInputStream(u)?.use { from -> to.outputStream().use { from.copyTo(it) } } ?: continue
                files.put(JSONObject().put("name", File(name).name).put("type", r.getType(u) ?: i.type ?: "").put("path", to.path))
            } catch (e: Exception) {
                // one file it couldn't read doesn't lose the rest
            }
        }
        if (text.isEmpty() && files.length() == 0) return
        val link = Regex("""https?://\S+""").find(text)?.value ?: ""
        synchronized(inbox) {
            inbox.add(JSONObject().put("text", text).put("url", link).put("subject", i.getStringExtra(Intent.EXTRA_SUBJECT) ?: "")
                .put("files", files).put("time", System.currentTimeMillis() / 1000.0))
        }
    }

    private fun take(most: Long): List<JSONObject> {
        val list = synchronized(inbox) { inbox.toList().also { inbox.clear() } }
        for (s in list) {
            val files = s.getJSONArray("files")
            for (n in 0 until files.length()) {
                val f = files.getJSONObject(n)
                val on = File(f.remove("path") as String)
                f.put("size", on.length())
                if (on.length() <= most) f.put("data", Base64.encodeToString(on.readBytes(), Base64.NO_WRAP))
                on.delete()
            }
        }
        return list
    }

    fun handle(call: PluginCall) {
        if (call.method != "take") return Plugins.unknownMethod(call)
        val most = call.args.optLong("maxBytes", 0).let { if (it > 0) it else 20L shl 20 }
        Thread {
            try {
                call.resolve(take(most))
            } catch (e: Exception) {
                call.reject(Plugins.FAILED, e.toString())
            }
        }.start()
    }
}
