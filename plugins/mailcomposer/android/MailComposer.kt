package space.nimbusgo.nativekit

import android.content.ContentProvider
import android.content.ContentValues
import android.content.Intent
import android.database.Cursor
import android.database.MatrixCursor
import android.net.Uri
import android.os.ParcelFileDescriptor
import android.provider.OpenableColumns
import android.util.Base64
import org.json.JSONArray
import java.io.File

/** Lets the mail app read an attachment, by its name. */
class MailFiles : ContentProvider() {
    private fun file(uri: Uri) = File(File(context!!.cacheDir, "nimbus-mail"), File(uri.lastPathSegment ?: "").name)

    override fun onCreate() = true
    override fun openFile(uri: Uri, mode: String): ParcelFileDescriptor = ParcelFileDescriptor.open(file(uri), ParcelFileDescriptor.MODE_READ_ONLY)
    override fun query(u: Uri, p: Array<String>?, s: String?, a: Array<String>?, o: String?): Cursor {
        val f = file(u)
        return MatrixCursor(arrayOf(OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE)).apply { addRow(arrayOf<Any>(f.name, f.length())) }
    }
    override fun getType(u: Uri): String? = u.getQueryParameter("type")
    override fun insert(u: Uri, v: ContentValues?): Uri? = null
    override fun delete(u: Uri, s: String?, a: Array<String>?) = 0
    override fun update(u: Uri, v: ContentValues?, s: String?, a: Array<String>?) = 0
}

/** mailcomposer: whichever mail app they use. It never says what happened. */
internal object MailComposerPlugin {
    private fun list(a: JSONArray?) = Array(a?.length() ?: 0) { a!!.getString(it) }

    fun handle(call: PluginCall) {
        when (call.method) {
            "available" -> call.resolve(mapOf("available" to true))
            "compose" -> compose(call)
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun compose(call: PluginCall) {
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        val mail = Intent(Intent.ACTION_SENDTO, Uri.parse("mailto:"))
        val files = call.args.optJSONArray("files")
        val uris = ArrayList<Uri>()
        if (files != null && files.length() > 0) {
            val dir = File(a.cacheDir, "nimbus-mail").apply { mkdirs() }
            for (i in 0 until files.length()) {
                val f = files.getJSONObject(i)
                val name = File(f.optString("name")).name
                File(dir, name).writeBytes(Base64.decode(f.optString("data"), Base64.DEFAULT))
                uris.add(Uri.parse("content://${a.packageName}.nimbusmail/").buildUpon().appendPath(name)
                    .appendQueryParameter("type", f.optString("type")).build())
            }
        }
        fun filled(i: Intent) = i.apply {
            putExtra(Intent.EXTRA_EMAIL, list(call.args.optJSONArray("to")))
            putExtra(Intent.EXTRA_CC, list(call.args.optJSONArray("cc")))
            putExtra(Intent.EXTRA_BCC, list(call.args.optJSONArray("bcc")))
            putExtra(Intent.EXTRA_SUBJECT, call.string("subject") ?: "")
            putExtra(Intent.EXTRA_TEXT, call.string("body") ?: "")
        }
        val done = { _: Int, _: Intent? -> call.resolve(mapOf("status" to "unknown")) }
        val none = { call.reject(Plugins.UNAVAILABLE, "this phone has no mail app"); Unit }
        if (uris.isEmpty()) return ResultAsker.start(a, filled(mail), none, done)
        // Attachments need a send, which any app can take: the selector asks for mail apps, and
        // where the phone won't honor it the chooser still gets the files out.
        val send = { filled(Intent(Intent.ACTION_SEND_MULTIPLE).setType("*/*")).apply {
            putParcelableArrayListExtra(Intent.EXTRA_STREAM, uris)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            clipData = android.content.ClipData.newRawUri("", uris[0])
        } }
        ResultAsker.start(a, send().apply { selector = mail }, {
            ResultAsker.start(a, Intent.createChooser(send(), null), none, done)
        }, done)
    }
}
