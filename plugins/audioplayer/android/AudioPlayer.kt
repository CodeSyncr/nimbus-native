package space.nimbusgo.nativekit

import android.content.Context
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.media.MediaPlayer
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.Base64
import java.io.File
import java.util.UUID

/** audioplayer: MediaPlayer, one for each sound. */
internal object AudioPlayerPlugin {
    private class Sound(val player: MediaPlayer) {
        var rate = 1f
        var ended = false
        var file: File? = null
    }

    private val sounds = HashMap<String, Sound>()
    private val audio get() = Plugins.context.getSystemService(Context.AUDIO_SERVICE) as AudioManager
    private val attributes = AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_MEDIA).setContentType(AudioAttributes.CONTENT_TYPE_MUSIC).build()
    private var focus: AudioFocusRequest? = null

    /** Another app's sound comes first: a call, say, pauses what's playing. */
    private fun takeFocus() {
        if (focus != null) return
        val f = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN).setAudioAttributes(attributes).setOnAudioFocusChangeListener { change ->
            if (change == AudioManager.AUDIOFOCUS_LOSS || change == AudioManager.AUDIOFOCUS_LOSS_TRANSIENT) {
                sounds.values.forEach { if (it.player.isPlaying) it.player.pause() }
            }
        }.build()
        focus = f
        audio.requestAudioFocus(f)
    }

    private fun letGo() {
        if (sounds.values.any { it.player.isPlaying }) return
        focus?.let { audio.abandonAudioFocusRequest(it) }
        focus = null
    }

    fun handle(call: PluginCall) {
        if (call.method == "open") return open(call)
        val s = sounds[call.string("id") ?: ""] ?: return call.reject(Plugins.NOT_FOUND, "that player is closed")
        val p = s.player
        when (call.method) {
            "play" -> {
                takeFocus()
                s.ended = false
                p.start()
                p.playbackParams = p.playbackParams.setSpeed(s.rate)
                call.resolve()
            }
            "pause" -> {
                if (p.isPlaying) p.pause()
                letGo()
                call.resolve()
            }
            "seek" -> {
                s.ended = false
                p.setOnSeekCompleteListener { call.resolve() }
                p.seekTo((call.args.optDouble("seconds", 0.0) * 1000).toInt())
            }
            "volume" -> {
                val v = call.args.optDouble("volume", 1.0).toFloat()
                p.setVolume(v, v)
                call.resolve()
            }
            "rate" -> {
                s.rate = call.args.optDouble("rate", 1.0).toFloat()
                if (p.isPlaying) p.playbackParams = p.playbackParams.setSpeed(s.rate)
                call.resolve()
            }
            "loop" -> {
                p.isLooping = call.args.optBoolean("loop")
                call.resolve()
            }
            "status" -> call.resolve(mapOf("position" to p.currentPosition / 1000.0, "duration" to maxOf(p.duration, 0) / 1000.0,
                "playing" to p.isPlaying, "ended" to s.ended))
            "close" -> {
                p.release()
                s.file?.delete()
                sounds.remove(call.string("id") ?: "")
                letGo()
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun open(call: PluginCall) {
        val p = MediaPlayer()
        val s = Sound(p)
        p.setAudioAttributes(attributes)
        try {
            val data = call.string("data")
            if (!data.isNullOrEmpty()) {
                val f = File.createTempFile("nimbus-sound", null, Plugins.context.cacheDir)
                f.writeBytes(Base64.decode(data, Base64.DEFAULT))
                s.file = f
                p.setDataSource(f.path)
            } else {
                val url = call.string("url")
                if (url.isNullOrEmpty()) return call.reject(Plugins.FAILED, "nothing to play")
                p.setDataSource(url)
            }
        } catch (e: java.io.IOException) {
            p.release()
            return call.reject(Plugins.FAILED, e.message ?: "it couldn't be played")
        }
        // Held from here on: a player nothing refers to can be collected mid-load, and never answers.
        val id = UUID.randomUUID().toString()
        sounds[id] = s
        val main = Handler(Looper.getMainLooper())
        val fail = { message: String ->
            main.removeCallbacksAndMessages(id)
            if (sounds.remove(id) != null) {
                s.file?.delete()
                p.release()
                call.reject(Plugins.FAILED, message)
            }
        }
        // A bad address can leave MediaPlayer waiting with no error at all.
        main.postAtTime({ fail("it took too long to load") }, id, SystemClock.uptimeMillis() + 30_000)
        p.setOnPreparedListener {
            main.removeCallbacksAndMessages(id)
            call.resolve(mapOf("id" to id))
        }
        p.setOnCompletionListener { s.ended = !p.isLooping }
        p.setOnErrorListener { _, what, extra ->
            fail("it couldn't be played ($what, $extra)")
            true
        }
        p.prepareAsync()
    }
}
