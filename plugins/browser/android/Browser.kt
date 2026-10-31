package space.nimbusgo.nativekit

import android.net.Uri
import androidx.browser.customtabs.CustomTabsIntent

/** browser: a Custom Tab. */
internal object BrowserPlugin {
    fun handle(call: PluginCall) {
        if (call.method != "open") return Plugins.unknownMethod(call)
        val u = Uri.parse(call.string("url") ?: "")
        if (u.scheme != "https" && u.scheme != "http") return call.reject(Plugins.FAILED, "that isn't a web address")
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        CustomTabsIntent.Builder().build().launchUrl(a, u)
        call.resolve()
    }
}
