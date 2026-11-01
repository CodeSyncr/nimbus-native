package space.nimbusgo.nativekit

import android.Manifest
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
import android.media.MediaMetadataRetriever
import android.net.Uri
import android.os.Build
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
                // An app that declares the camera permission must hold it to use the camera app,
                // or Android throws: camera-view declares it.
                val declared = a.packageManager.getPackageInfo(a.packageName, PackageManager.GET_PERMISSIONS)
                    .requestedPermissions?.contains(Manifest.permission.CAMERA) == true
                if (declared && a.checkSelfPermission(Manifest.permission.CAMERA) != PackageManager.PERMISSION_GRANTED) {
                    return PermissionAsker.ask(a, Manifest.permission.CAMERA) { ok ->
                        if (ok) take(call, a) else call.reject(Plugins.DENIED, "they said no")
                    }
                }
                take(call, a)
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
            "pickMany" -> pickMany(call, a)
            "pickVideo" -> pickVideo(call, a)
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun take(call: PluginCall, a: Activity) {
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

    /** Reads, turns and sizes the picture off the main thread. */
    private fun respond(call: PluginCall, open: () -> Pair<InputStream, InputStream>) {
        thread {
            try {
                call.resolve(photo(call, open))
            } catch (e: Exception) {
                call.reject(Plugins.FAILED, e.toString())
            }
        }
    }

    private fun photo(call: PluginCall, open: () -> Pair<InputStream, InputStream>): Map<String, Any> {
        val (forTurn, forImage) = open()
        val turn = forTurn.use {
            when (ExifInterface(it).getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL)) {
                ExifInterface.ORIENTATION_ROTATE_90 -> 90f
                ExifInterface.ORIENTATION_ROTATE_180 -> 180f
                ExifInterface.ORIENTATION_ROTATE_270 -> 270f
                else -> 0f
            }
        }
        val source = forImage.use { BitmapFactory.decodeStream(it) } ?: throw java.io.IOException("the photo couldn't be read")
        val limit = call.args.optInt("maxSize", 0).let { if (it > 0) it else 2000 }
        val scale = minOf(1f, limit.toFloat() / maxOf(source.width, source.height))
        val m = Matrix().apply { postScale(scale, scale); postRotate(turn) }
        val out = Bitmap.createBitmap(source, 0, 0, source.width, source.height, m, true)
        val q = call.args.optDouble("quality", 0.0).let { if (it > 0) it else 0.85 }
        val b = ByteArrayOutputStream()
        out.compress(Bitmap.CompressFormat.JPEG, (q * 100).toInt(), b)
        return mapOf("data" to Base64.encodeToString(b.toByteArray(), Base64.NO_WRAP), "width" to out.width, "height" to out.height)
    }

    /** The system's photo picker from Android 13, and the file picker before it. */
    private fun library(type: String, limit: Int): Intent {
        if (Build.VERSION.SDK_INT >= 33) {
            val i = Intent(MediaStore.ACTION_PICK_IMAGES).setType(type)
            if (limit != 1) i.putExtra(MediaStore.EXTRA_PICK_IMAGES_MAX, if (limit > 1) minOf(limit, MediaStore.getPickImagesMaxLimit()) else MediaStore.getPickImagesMaxLimit())
            return i
        }
        return Intent(Intent.ACTION_GET_CONTENT).setType(type).addCategory(Intent.CATEGORY_OPENABLE)
            .putExtra(Intent.EXTRA_ALLOW_MULTIPLE, limit != 1)
    }

    private fun pickMany(call: PluginCall, a: Activity) {
        val limit = call.args.optInt("limit", 0)
        ResultAsker.start(a, library("image/*", limit)) { result, data ->
            val clip = data?.clipData
            val uris = if (clip != null) List(clip.itemCount) { clip.getItemAt(it).uri } else listOfNotNull(data?.data)
            if (result != Activity.RESULT_OK || uris.isEmpty()) return@start call.reject(Plugins.CANCELLED, "no photos")
            val r = a.contentResolver
            thread {
                try {
                    // The older picker can't be told a limit, so it's applied here.
                    val keep = if (limit > 0) uris.take(limit) else uris
                    call.resolve(keep.map { u -> photo(call) { r.openInputStream(u)!! to r.openInputStream(u)!! } })
                } catch (e: Exception) {
                    call.reject(Plugins.FAILED, e.toString())
                }
            }
        }
    }

    private fun pickVideo(call: PluginCall, a: Activity) {
        ResultAsker.start(a, library("video/*", 1)) { result, data ->
            val uri = data?.data ?: data?.clipData?.takeIf { it.itemCount > 0 }?.getItemAt(0)?.uri
            if (result != Activity.RESULT_OK || uri == null) return@start call.reject(Plugins.CANCELLED, "no video")
            thread {
                try {
                    val r = a.contentResolver
                    val limit = call.args.optLong("maxBytes", 50L shl 20)
                    val size = r.openAssetFileDescriptor(uri, "r")?.use { it.length } ?: -1
                    if (size > limit) return@thread call.reject(Plugins.FAILED, "the video is larger than MaxBytes")
                    val bytes = r.openInputStream(uri)!!.use { it.readBytes() }
                    if (bytes.size > limit) return@thread call.reject(Plugins.FAILED, "the video is larger than MaxBytes")
                    val m = MediaMetadataRetriever()
                    val ms = try {
                        m.setDataSource(a, uri)
                        m.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)?.toDoubleOrNull() ?: 0.0
                    } finally {
                        m.release()
                    }
                    call.resolve(mapOf("data" to Base64.encodeToString(bytes, Base64.NO_WRAP), "type" to (r.getType(uri) ?: "video/mp4"), "seconds" to ms / 1000))
                } catch (e: Exception) {
                    call.reject(Plugins.FAILED, e.toString())
                }
            }
        }
    }
}
