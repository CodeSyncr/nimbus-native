package space.nimbusgo.nativekit

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Bundle
import android.speech.RecognitionListener
import android.speech.RecognizerIntent
import android.speech.SpeechRecognizer

/** speechrecognition: the platform's recognizer. */
internal object SpeechRecognitionPlugin {
    private var recognizer: SpeechRecognizer? = null
    private var text = ""
    private var done = false
    private var failure: String? = null

    private fun words(b: Bundle?) = b?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)?.firstOrNull() ?: ""

    private val listener = object : RecognitionListener {
        override fun onPartialResults(b: Bundle?) { words(b).let { if (it.isNotEmpty()) text = it } }
        override fun onResults(b: Bundle?) {
            words(b).let { if (it.isNotEmpty()) text = it }
            done = true
        }
        override fun onError(code: Int) {
            // Nothing heard ends a listen; the rest are real failures.
            if (code != SpeechRecognizer.ERROR_NO_MATCH && code != SpeechRecognizer.ERROR_SPEECH_TIMEOUT) failure = "the recognizer failed ($code)"
            done = true
        }
        override fun onReadyForSpeech(b: Bundle?) {}
        override fun onBeginningOfSpeech() {}
        override fun onRmsChanged(v: Float) {}
        override fun onBufferReceived(b: ByteArray?) {}
        override fun onEndOfSpeech() {}
        override fun onEvent(t: Int, b: Bundle?) {}
    }

    fun handle(call: PluginCall) {
        val c = Plugins.context
        when (call.method) {
            "request" -> {
                if (c.checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED) return call.resolve(mapOf("granted" to true))
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                PermissionAsker.ask(a, Manifest.permission.RECORD_AUDIO) { call.resolve(mapOf("granted" to it)) }
            }
            "start" -> {
                if (!SpeechRecognizer.isRecognitionAvailable(c)) return call.reject(Plugins.UNAVAILABLE, "speech recognition isn't available here")
                if (c.checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) return call.reject(Plugins.DENIED, "the microphone is not allowed")
                text = ""
                done = false
                failure = null
                recognizer?.destroy()
                recognizer = SpeechRecognizer.createSpeechRecognizer(c).also { it.setRecognitionListener(listener) }
                val i = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true)
                    .putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
                call.string("language")?.takeIf { it.isNotEmpty() }?.let { i.putExtra(RecognizerIntent.EXTRA_LANGUAGE, it) }
                recognizer?.startListening(i)
                call.resolve()
            }
            "read" -> {
                val f = failure
                if (f != null) {
                    failure = null
                    return call.reject(Plugins.FAILED, f)
                }
                call.resolve(mapOf("text" to text, "done" to done))
            }
            "stop" -> {
                recognizer?.cancel()
                done = true
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
