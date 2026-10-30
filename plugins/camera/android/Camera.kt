package space.nimbusgo.nativekit

import android.app.Activity
import android.content.ContentProvider
import android.content.ContentValues
import android.content.Intent
import android.content.pm.PackageManager
import android.database.Cursor
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Matrix
import android.media.ExifInterface
import android.net.Uri
import android.os.ParcelFileDescriptor
import android.provider.MediaStore
import android.util.Base64
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.InputStream
import kotlin.concurrent.thread

/** Lets the camera app write its photo into this app's cache. */
class CameraFiles : ContentProvider() {
    override fun onCreate() = true
    override fun openFile(uri: Uri, mode: String): ParcelFileDescriptor {
        val f = File(context!!.cacheDir, "nimbus-photo.jpg")
        return ParcelFileDescriptor.open(f, ParcelFileDescriptor.MODE_READ_WRITE or ParcelFileDescriptor.MODE_CREATE)
    }
    override fun query(u: Uri, p: Array<String>?, s: String?, a: Array<String>?, o: String?): Cursor? = null
    override fun getType(u: Uri) = "image/jpeg"
    override fun insert(u: Uri, v: ContentValues?): Uri? = null
    override fun delete(u: Uri, s: String?, a: Array<String>?) = 0
    override fun update(u: Uri, v: ContentValues?, s: String?, a: Array<String>?) = 0
}

/** camera: the system's camera and photo picker, as a JPEG the right way up. */
internal object CameraPlugin {
    fun handle(call: PluginCall) {
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        when (call.method) {
            "take" -> {
                if (!a.packageManager.hasSystemFeature(PackageManager.FEATURE_CAMERA_ANY)) return call.reject(Plugins.UNAVAILABLE, "this phone has no camera")
                val file = File(a.cacheDir, "nimbus-photo.jpg")
                file.delete()
                val uri = Uri.parse("content://${a.packageName}.nimbuscamera/photo.jpg")
                val i = Intent(MediaStore.ACTION_IMAGE_CAPTURE).putExtra(MediaStore.EXTRA_OUTPUT, uri)
                    .addFlags(Intent.FLAG_GRANT_WRITE_URI_PERMISSION or Intent.FLAG_GRANT_READ_URI_PERMISSION)
                ResultAsker.start(a, i) { result, _ ->
                    if (result != Activity.RESULT_OK || !file.exists()) return@start call.reject(Plugins.CANCELLED, "no photo")
                    respond(call) { file.inputStream() to file.inputStream() }
                }
            }
            "pick" -> {
                val i = Intent(Intent.ACTION_GET_CONTENT).setType("image/*").addCategory(Intent.CATEGORY_OPENABLE)
                ResultAsker.start(a, i) { result, data ->
                    val uri = data?.data
                    if (result != Activity.RESULT_OK || uri == null) return@start call.reject(Plugins.CANCELLED, "no photo")
                    val r = a.contentResolver
                    respond(call) { r.openInputStream(uri)!! to r.openInputStream(uri)!! }
                }
            }
            else -> Plugins.unknownMethod(call)
        }
    }

    /** Reads, turns and sizes the picture off the main thread. */
    private fun respond(call: PluginCall, open: () -> Pair<InputStream, InputStream>) {
        thread {
            try {
                val (forTurn, forImage) = open()
                val turn = forTurn.use {
                    when (ExifInterface(it).getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL)) {
                        ExifInterface.ORIENTATION_ROTATE_90 -> 90f
                        ExifInterface.ORIENTATION_ROTATE_180 -> 180f
                        ExifInterface.ORIENTATION_ROTATE_270 -> 270f
                        else -> 0f
                    }
                }
                val source = forImage.use { BitmapFactory.decodeStream(it) } ?: return@thread call.reject(Plugins.FAILED, "that isn't a picture")
                val limit = call.args.optInt("maxSize", 0).let { if (it > 0) it else 2000 }
                val scale = minOf(1f, limit.toFloat() / maxOf(source.width, source.height))
                val m = Matrix().apply { postScale(scale, scale); postRotate(turn) }
                val out = Bitmap.createBitmap(source, 0, 0, source.width, source.height, m, true)
                val q = call.args.optDouble("quality", 0.0).let { if (it > 0) it else 0.85 }
                val b = ByteArrayOutputStream()
                out.compress(Bitmap.CompressFormat.JPEG, (q * 100).toInt(), b)
                call.resolve(mapOf("data" to Base64.encodeToString(b.toByteArray(), Base64.NO_WRAP), "width" to out.width, "height" to out.height))
            } catch (e: Exception) {
                call.reject(Plugins.FAILED, e.toString())
            }
        }
    }
}
