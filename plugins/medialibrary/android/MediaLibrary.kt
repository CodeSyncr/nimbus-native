package space.nimbusgo.nativekit

import android.Manifest
import android.content.ContentUris
import android.content.ContentValues
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.net.Uri
import android.os.Build
import android.provider.MediaStore
import android.util.Base64
import android.util.Size
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import kotlin.concurrent.thread

/** medialibrary: the media store. Saving needs no permission from Android 10. */
internal object MediaLibraryPlugin {
    private val images = MediaStore.Images.Media.EXTERNAL_CONTENT_URI
    private val videos = MediaStore.Video.Media.EXTERNAL_CONTENT_URI

    private val needed get() = if (Build.VERSION.SDK_INT >= 33)
        listOf(Manifest.permission.READ_MEDIA_IMAGES, Manifest.permission.READ_MEDIA_VIDEO)
    else listOf(Manifest.permission.READ_EXTERNAL_STORAGE)

    private fun missing() = needed.filter { Plugins.context.checkSelfPermission(it) != PackageManager.PERMISSION_GRANTED }

    fun handle(call: PluginCall) {
        when (call.method) {
            "request" -> request(call)
            "save" -> thread { save(call) }
            "list" -> if (missing().isNotEmpty()) call.reject(Plugins.DENIED, "photos aren't allowed: call Request first") else thread { list(call) }
            "read" -> thread { read(call) }
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun request(call: PluginCall) {
        val next = missing().firstOrNull() ?: return call.resolve(mapOf("granted" to true))
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
        PermissionAsker.ask(a, next) { ok -> if (ok) request(call) else call.resolve(mapOf("granted" to false)) }
    }

    private fun save(call: PluginCall) {
        if (Build.VERSION.SDK_INT < 29) return call.reject(Plugins.UNAVAILABLE, "saving needs Android 10 or later")
        val type = call.string("type") ?: "image/jpeg"
        val video = type.startsWith("video/")
        val v = ContentValues().apply {
            put(MediaStore.MediaColumns.DISPLAY_NAME, call.string("name") ?: "image")
            put(MediaStore.MediaColumns.MIME_TYPE, type)
            put(MediaStore.MediaColumns.IS_PENDING, 1)
        }
        val r = Plugins.context.contentResolver
        val uri = r.insert(if (video) videos else images, v) ?: return call.reject(Plugins.FAILED, "it couldn't be saved")
        try {
            r.openOutputStream(uri)!!.use { it.write(Base64.decode(call.string("data") ?: "", Base64.DEFAULT)) }
            r.update(uri, ContentValues().apply { put(MediaStore.MediaColumns.IS_PENDING, 0) }, null, null)
            call.resolve(mapOf("id" to uri.toString()))
        } catch (e: java.io.IOException) {
            r.delete(uri, null, null)
            call.reject(Plugins.FAILED, e.message ?: "it couldn't be saved")
        }
    }

    private fun query(uri: Uri, video: Boolean, limit: Int, out: MutableList<JSONObject>) {
        val m = MediaStore.MediaColumns._ID
        val columns = arrayOf(m, MediaStore.MediaColumns.WIDTH, MediaStore.MediaColumns.HEIGHT, MediaStore.MediaColumns.DATE_ADDED)
        Plugins.context.contentResolver.query(uri, columns, null, null, "${MediaStore.MediaColumns.DATE_ADDED} DESC")?.use {
            var n = 0
            while (n++ < limit && it.moveToNext()) {
                out.add(JSONObject().put("id", ContentUris.withAppendedId(uri, it.getLong(0)).toString()).put("video", video)
                    .put("width", it.getInt(1)).put("height", it.getInt(2)).put("time", it.getLong(3)))
            }
        }
    }

    private fun list(call: PluginCall) {
        val limit = call.args.optInt("limit", 50)
        val all = ArrayList<JSONObject>()
        query(images, false, limit, all)
        query(videos, true, limit, all)
        call.resolve(all.sortedByDescending { it.getLong("time") }.take(limit))
    }

    private fun read(call: PluginCall) {
        val uri = Uri.parse(call.string("id") ?: "")
        val max = call.args.optInt("maxSize", 0)
        val r = Plugins.context.contentResolver
        try {
            val bitmap = if (Build.VERSION.SDK_INT >= 29 && (max > 0 || uri.toString().contains("/video/")))
                r.loadThumbnail(uri, Size(if (max > 0) max else 2048, if (max > 0) max else 2048), null)
            else r.openInputStream(uri)!!.use { BitmapFactory.decodeStream(it) }
            val b = ByteArrayOutputStream()
            bitmap.compress(Bitmap.CompressFormat.JPEG, 85, b)
            call.resolve(mapOf("data" to Base64.encodeToString(b.toByteArray(), Base64.NO_WRAP)))
        } catch (e: java.io.IOException) {
            call.reject(Plugins.NOT_FOUND, "no such item")
        } catch (e: SecurityException) {
            call.reject(Plugins.DENIED, "photos aren't allowed: call Request first")
        }
    }
}
