package space.nimbusgo.nativekit

import android.content.Intent

/** share: the chooser. */
internal object SharePlugin {
    fun handle(call: PluginCall) {
        if (call.method != "share") return Plugins.unknownMethod(call)
        val text = listOfNotNull(call.string("text"), call.string("url")).joinToString("\n")
        if (text.isEmpty()) return call.reject(Plugins.FAILED, "nothing to share")
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to share from")
        val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, text)
        call.string("title")?.let { send.putExtra(Intent.EXTRA_SUBJECT, it) }
        a.startActivity(Intent.createChooser(send, call.string("title")))
        call.resolve()
    }
}
