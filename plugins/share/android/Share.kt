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
import java.io.File

/** Lets the app that receives a share read the file, by its name. */
class ShareFiles : ContentProvider() {
    private fun file(uri: Uri) = File(File(context!!.cacheDir, "nimbus-share"), File(uri.lastPathSegment ?: "").name)

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

/** share: the chooser. */
internal object SharePlugin {
    fun handle(call: PluginCall) {
        if (call.method != "share") return Plugins.unknownMethod(call)
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to share from")
        val text = listOfNotNull(call.string("text"), call.string("url")).joinToString("\n")
        val dir = File(a.cacheDir, "nimbus-share").apply { mkdirs() }
        val uris = ArrayList<Uri>()
        var type = ""
        val files = call.args.optJSONArray("files")
        for (i in 0 until (files?.length() ?: 0)) {
            val f = files!!.getJSONObject(i)
            val name = File(f.optString("name")).name
            File(dir, name).writeBytes(Base64.decode(f.optString("data"), Base64.DEFAULT))
            val t = f.optString("type", "*/*").ifEmpty { "*/*" }
            type = if (type.isEmpty() || type == t) t else "*/*"
            uris.add(Uri.parse("content://${a.packageName}.nimbusshare/").buildUpon().appendPath(name).appendQueryParameter("type", t).build())
        }
        if (text.isEmpty() && uris.isEmpty()) return call.reject(Plugins.FAILED, "nothing to share")
        val send = Intent(if (uris.size > 1) Intent.ACTION_SEND_MULTIPLE else Intent.ACTION_SEND).setType(type.ifEmpty { "text/plain" })
        if (text.isNotEmpty()) send.putExtra(Intent.EXTRA_TEXT, text)
        call.string("title")?.let { send.putExtra(Intent.EXTRA_SUBJECT, it) }
        if (uris.size == 1) send.putExtra(Intent.EXTRA_STREAM, uris[0])
        if (uris.size > 1) send.putParcelableArrayListExtra(Intent.EXTRA_STREAM, uris)
        if (uris.isNotEmpty()) {
            send.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            send.clipData = android.content.ClipData.newRawUri("", uris[0])
        }
        a.startActivity(Intent.createChooser(send, call.string("title")).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION))
        call.resolve()
    }
}
