package space.nimbusgo.nativekit

import android.Manifest
import android.content.pm.PackageManager
import android.media.MediaMetadataRetriever
import android.media.MediaRecorder
import android.os.Build
import android.os.SystemClock
import android.util.Base64
import java.io.File

/** audiorecorder: MediaRecorder, to an AAC file in the cache. */
internal object AudioRecorderPlugin {
    private var recorder: MediaRecorder? = null
    private var file: File? = null
    private var paused = false
    private var since = 0L // when the current stretch began
    private var banked = 0L // what's recorded before it

    private fun allowed() = Plugins.context.checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED

    private fun elapsed() = banked + if (paused || recorder == null) 0 else SystemClock.elapsedRealtime() - since

    private fun release() {
        recorder?.release()
        recorder = null
        paused = false
        banked = 0
    }

    @Suppress("DEPRECATION")
    private fun start(call: PluginCall) {
        if (!allowed()) return call.reject(Plugins.DENIED, "the microphone isn't allowed: call Request first")
        if (recorder != null) return call.reject(Plugins.FAILED, "it's already recording")
        val f = File.createTempFile("nimbus-recording", ".m4a", Plugins.context.cacheDir)
        val r = if (Build.VERSION.SDK_INT >= 31) MediaRecorder(Plugins.context) else MediaRecorder()
        try {
            r.setAudioSource(MediaRecorder.AudioSource.MIC)
            r.setOutputFormat(MediaRecorder.OutputFormat.MPEG_4)
            r.setAudioEncoder(MediaRecorder.AudioEncoder.AAC)
            r.setAudioEncodingBitRate(128_000)
            r.setAudioSamplingRate(44_100)
            r.setOutputFile(f.path)
            r.prepare()
            r.start()
        } catch (e: Exception) {
            r.release()
            f.delete()
            return call.reject(Plugins.FAILED, e.message ?: "the microphone wouldn't start")
        }
        recorder = r
        file = f
        since = SystemClock.elapsedRealtime()
        call.resolve()
    }

    fun handle(call: PluginCall) {
        when (call.method) {
            "request" -> {
                if (allowed()) return call.resolve(mapOf("granted" to true))
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                PermissionAsker.ask(a, Manifest.permission.RECORD_AUDIO) { call.resolve(mapOf("granted" to it)) }
            }
            "start" -> start(call)
            "pause" -> {
                val r = recorder ?: return call.reject(Plugins.FAILED, "nothing is recording")
                if (!paused) {
                    r.pause()
                    banked = elapsed()
                    paused = true
                }
                call.resolve()
            }
            "resume" -> {
                val r = recorder ?: return call.reject(Plugins.FAILED, "nothing is recording")
                if (paused) {
                    r.resume()
                    since = SystemClock.elapsedRealtime()
                    paused = false
                }
                call.resolve()
            }
            "status" -> call.resolve(mapOf("recording" to (recorder != null && !paused), "seconds" to elapsed() / 1000.0,
                "level" to if (recorder != null && !paused) recorder!!.maxAmplitude / 32767.0 else 0.0))
            "stop" -> stop(call)
            "cancel" -> {
                try { recorder?.stop() } catch (e: RuntimeException) {}
                release()
                file?.delete()
                file = null
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun stop(call: PluginCall) {
        val r = recorder ?: return call.reject(Plugins.FAILED, "nothing is recording")
        val f = file
        try {
            r.stop()
        } catch (e: RuntimeException) { // stopped before anything was captured
            release()
            f?.delete()
            return call.reject(Plugins.FAILED, "too short to keep")
        }
        release()
        val m = MediaMetadataRetriever()
        val ms = try {
            m.setDataSource(f!!.path)
            m.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)?.toDoubleOrNull() ?: 0.0
        } finally {
            m.release()
        }
        val bytes = f!!.readBytes()
        f.delete()
        file = null
        call.resolve(mapOf("data" to Base64.encodeToString(bytes, Base64.NO_WRAP), "type" to "audio/mp4", "seconds" to ms / 1000))
    }
}
