package space.nimbusgo.nativekit

import android.Manifest
import android.app.Activity
import android.content.Context
import android.content.pm.PackageManager
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioFormat
import android.media.AudioManager
import android.media.AudioRecord
import android.media.MediaPlayer
import android.media.MediaRecorder
import android.media.audiofx.AcousticEchoCanceler
import android.media.audiofx.AutomaticGainControl
import android.media.audiofx.NoiseSuppressor
import android.media.audiofx.Visualizer
import android.util.Base64
import java.io.ByteArrayOutputStream
import java.io.File
import kotlin.concurrent.thread

/**
 * audio: the microphone, as 16 kHz mono PCM for Go to read, and a player. The microphone uses the
 * voice-call source with the phone's noise suppression and echo cancelling, but not its level
 * control: that lifts a voice across the room to sound like the one at the phone, and loudness is
 * how a turn's speaker is told apart. Needs RECORD_AUDIO and MODIFY_AUDIO_SETTINGS in the app's
 * manifest; the player's level needs them too.
 */
internal object Audio {
    private const val RATE = 16000
    private const val LIMIT = 960_000 // thirty seconds; older audio goes if Go stops reading

    private var recorder: AudioRecord? = null
    private val pcm = ByteArrayOutputStream()

    private var player: MediaPlayer? = null
    private var meter: Visualizer? = null
    private var loudest = -9600
    private var focus: AudioFocusRequest? = null

    private val manager get() = Plugins.context.getSystemService(Context.AUDIO_SERVICE) as AudioManager

    fun handle(call: PluginCall) {
        when (call.method) {
            "requestMicrophone" -> requestMicrophone(call)
            "micStart" -> micStart(call)
            "micRead" -> {
                val bytes = synchronized(pcm) { pcm.toByteArray().also { pcm.reset() } }
                call.resolve(mapOf("pcm" to Base64.encodeToString(bytes, Base64.NO_WRAP)))
            }
            "micStop" -> {
                micStop()
                call.resolve()
            }
            "play" -> play(call)
            "playing" -> {
                val p = player ?: return call.resolve(mapOf("playing" to false, "level" to 0))
                if (!p.isPlaying) {
                    stopPlayer()
                    return call.resolve(mapOf("playing" to false, "level" to 0))
                }
                call.resolve(mapOf("playing" to true, "level" to level()))
            }
            "stop" -> {
                stopPlayer()
                call.resolve()
            }
            "release" -> {
                micStop()
                stopPlayer()
                focus?.let { manager.abandonAudioFocusRequest(it) }
                focus = null
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun declared(): Boolean {
        val c = Plugins.context
        val info = c.packageManager.getPackageInfo(c.packageName, PackageManager.GET_PERMISSIONS)
        return info.requestedPermissions?.contains(Manifest.permission.RECORD_AUDIO) == true
    }

    private fun allowed() = Plugins.context.checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED

    private fun requestMicrophone(call: PluginCall) {
        if (!declared()) return call.reject(Plugins.UNAVAILABLE, "add RECORD_AUDIO to the app's manifest")
        if (allowed()) return call.resolve(mapOf("granted" to true))
        val activity = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
        PermissionAsker.ask(activity, Manifest.permission.RECORD_AUDIO) { call.resolve(mapOf("granted" to it)) }
    }

    private fun takeFocus() {
        if (focus != null) return
        val request = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN_TRANSIENT_MAY_DUCK)
            .setAudioAttributes(speech).build()
        manager.requestAudioFocus(request)
        focus = request
    }

    private val speech = AudioAttributes.Builder()
        .setUsage(AudioAttributes.USAGE_ASSISTANT)
        .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
        .build()

    @android.annotation.SuppressLint("MissingPermission") // allowed() is checked first
    private fun micStart(call: PluginCall) {
        if (recorder != null) return call.resolve()
        if (!allowed()) return call.reject(Plugins.DENIED, "the microphone is not allowed")
        val min = AudioRecord.getMinBufferSize(RATE, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT)
        val r = AudioRecord(
            MediaRecorder.AudioSource.VOICE_COMMUNICATION, RATE, AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT, maxOf(min, RATE),
        )
        if (r.state != AudioRecord.STATE_INITIALIZED) {
            r.release()
            return call.reject(Plugins.FAILED, "the microphone couldn't start")
        }
        val session = r.audioSessionId
        if (NoiseSuppressor.isAvailable()) NoiseSuppressor.create(session)?.enabled = true
        if (AcousticEchoCanceler.isAvailable()) AcousticEchoCanceler.create(session)?.enabled = true
        if (AutomaticGainControl.isAvailable()) AutomaticGainControl.create(session)?.enabled = false
        takeFocus()
        synchronized(pcm) { pcm.reset() }
        r.startRecording()
        recorder = r
        thread(name = "nimbus-audio") {
            val chunk = ByteArray(1024)
            while (true) {
                val n = r.read(chunk, 0, chunk.size)
                if (n <= 0) break
                synchronized(pcm) {
                    if (pcm.size() + n > LIMIT) {
                        val kept = pcm.toByteArray()
                        pcm.reset()
                        pcm.write(kept, kept.size / 2, kept.size - kept.size / 2)
                    }
                    pcm.write(chunk, 0, n)
                }
            }
            r.release()
        }
        call.resolve()
    }

    private fun micStop() {
        val r = recorder ?: return
        recorder = null
        r.stop() // the reading thread ends, and releases it
        synchronized(pcm) { pcm.reset() }
    }

    private fun play(call: PluginCall) {
        val clip = try {
            Base64.decode(call.string("clip") ?: "", Base64.DEFAULT)
        } catch (e: IllegalArgumentException) {
            return call.reject(Plugins.FAILED, "clip is not base64")
        }
        stopPlayer()
        takeFocus()
        // MediaPlayer reads from a file.
        val file = File.createTempFile("clip", null, Plugins.context.cacheDir)
        file.writeBytes(clip)
        val p = MediaPlayer()
        p.setAudioAttributes(speech)
        p.setDataSource(file.path)
        p.setOnPreparedListener {
            file.delete()
            if (player !== p) return@setOnPreparedListener call.resolve() // stopped meanwhile
            p.start()
            meter = openMeter(p)
            loudest = -9600
            call.resolve() // only now: Go's first "playing" must see it started
        }
        p.setOnErrorListener { _, what, extra ->
            file.delete()
            if (player === p) stopPlayer()
            call.reject(Plugins.FAILED, "the clip couldn't play ($what, $extra)")
            true
        }
        player = p
        p.prepareAsync()
    }

    private fun openMeter(p: MediaPlayer): Visualizer? = try {
        Visualizer(p.audioSessionId).apply {
            measurementMode = Visualizer.MEASUREMENT_MODE_PEAK_RMS
            enabled = true
        }
    } catch (e: Exception) {
        null // no permission, or no such effect here: the level stays 0
    }

    private fun level(): Float {
        val m = meter ?: return 0f
        val out = Visualizer.MeasurementPeakRms()
        if (m.getMeasurementPeakRms(out) != Visualizer.SUCCESS) return 0f
        // Measured after the volume, so against this clip's loudest moment so far, not full scale.
        loudest = maxOf(loudest, out.mRms)
        return (((out.mRms - loudest) / 100f + 30f) / 30f).coerceIn(0f, 1f) // mRms is in millibels
    }

    private fun stopPlayer() {
        meter?.release()
        meter = null
        player?.release()
        player = null
    }
}

/** Asks for a permission from a plain Activity: the answer comes to a fragment, not the app's own code. */
@Suppress("DEPRECATION")
class PermissionAsker : android.app.Fragment() {
    private var permission = ""
    private var done: ((Boolean) -> Unit)? = null

    override fun onCreate(state: android.os.Bundle?) {
        super.onCreate(state)
        if (done == null) {
            fragmentManager.beginTransaction().remove(this).commit() // rebuilt after a restart: nobody is waiting
            return
        }
        requestPermissions(arrayOf(permission), 1)
    }

    override fun onRequestPermissionsResult(code: Int, permissions: Array<String>, results: IntArray) {
        done?.invoke(results.firstOrNull() == PackageManager.PERMISSION_GRANTED)
        done = null
        fragmentManager.beginTransaction().remove(this).commit()
    }

    internal companion object {
        fun ask(activity: Activity, permission: String, done: (Boolean) -> Unit) {
            val asker = PermissionAsker()
            asker.permission = permission
            asker.done = done
            activity.fragmentManager.beginTransaction().add(asker, "nimbus-permission").commit()
        }
    }
}
