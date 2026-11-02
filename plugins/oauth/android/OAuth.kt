package space.nimbusgo.nativekit

import android.app.Activity
import android.app.Application
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.browser.customtabs.CustomTabsIntent

/** The provider sends them back to the app's scheme, which opens this. It hands the address over and gets out of the way. */
class OAuthRedirectActivity : Activity() {
    override fun onCreate(saved: Bundle?) {
        super.onCreate(saved)
        intent?.data?.let { OAuthPlugin.arrived(it) }
        // Back to the app, closing the Custom Tab that sat on top of it.
        packageManager.getLaunchIntentForPackage(packageName)?.let {
            startActivity(it.addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP))
        }
        finish()
    }
}

/** oauth: the provider's page in a Custom Tab. Go does the rest: PKCE, state, the exchange. */
internal object OAuthPlugin {
    private var waiting: PluginCall? = null
    private var scheme = ""
    private var from: Activity? = null
    private var left = false // the app went behind the tab

    /** Coming back to the app without the redirect means they closed the tab. */
    private val watch = object : Application.ActivityLifecycleCallbacks {
        override fun onActivityPaused(a: Activity) { if (a === from) left = true }
        override fun onActivityResumed(a: Activity) {
            if (a === from && left) finish()?.reject(Plugins.CANCELLED, "they backed out")
        }
        override fun onActivityCreated(a: Activity, b: Bundle?) {}
        override fun onActivityStarted(a: Activity) {}
        override fun onActivityStopped(a: Activity) {}
        override fun onActivitySaveInstanceState(a: Activity, b: Bundle) {}
        override fun onActivityDestroyed(a: Activity) { if (a === from) finish()?.reject(Plugins.CANCELLED, "the app's screen closed") }
    }

    private fun finish(): PluginCall? {
        val c = waiting ?: return null
        from?.application?.unregisterActivityLifecycleCallbacks(watch)
        waiting = null
        from = null
        return c
    }

    fun arrived(u: Uri) {
        if (!u.scheme.equals(scheme, ignoreCase = true)) return
        finish()?.resolve(mapOf("url" to u.toString()))
    }

    fun handle(call: PluginCall) {
        if (call.method != "open") return Plugins.unknownMethod(call)
        val page = Uri.parse(call.string("url") ?: "")
        if (page.scheme != "https" && page.scheme != "http") return call.reject(Plugins.FAILED, "that isn't a web address")
        val s = call.string("scheme")
        if (s.isNullOrEmpty()) return call.reject(Plugins.FAILED, "no redirect scheme")
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        finish()?.reject(Plugins.CANCELLED, "another sign-in started") // one at a time
        waiting = call
        scheme = s
        from = a
        left = false
        a.application.registerActivityLifecycleCallbacks(watch)
        CustomTabsIntent.Builder().build().launchUrl(a, page)
    }
}
