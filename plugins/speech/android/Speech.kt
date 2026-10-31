package space.nimbusgo.nativekit

import android.speech.tts.TextToSpeech
import java.util.Locale

/** speech: the platform's text to speech, started when first asked. */
internal object SpeechPlugin {
    private var tts: TextToSpeech? = null
    private var ready = false

    private fun speak(call: PluginCall) {
        val t = tts ?: return
        val l = call.string("language") ?: ""
        if (l.isNotEmpty()) t.language = Locale.forLanguageTag(l)
        val r = call.args.optDouble("rate", 0.0)
        t.setSpeechRate(if (r > 0) r.toFloat() else 1f)
        if (t.speak(call.string("text") ?: "", TextToSpeech.QUEUE_ADD, null, "nimbus") == TextToSpeech.SUCCESS) call.resolve()
        else call.reject(Plugins.FAILED, "the phone couldn't speak")
    }

    fun handle(call: PluginCall) {
        when (call.method) {
            "say" -> {
                if (ready) return speak(call)
                tts = TextToSpeech(Plugins.context) { status ->
                    if (status == TextToSpeech.SUCCESS) {
                        ready = true
                        speak(call)
                    } else {
                        tts = null
                        call.reject(Plugins.UNAVAILABLE, "this phone has no speech engine")
                    }
                }
            }
            "speaking" -> call.resolve(mapOf("speaking" to (tts?.isSpeaking == true)))
            "stop" -> {
                tts?.stop()
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
