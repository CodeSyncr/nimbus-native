package space.nimbusgo.nativekit

import android.annotation.SuppressLint
import android.content.Context
import android.view.View
import android.webkit.WebView
import android.webkit.WebViewClient
import org.json.JSONObject

/** webview: the platform's WebView as a native view (ui.Native). */
internal object WebViewPlugin {
    fun boot() {
        NativeViews.register("webview") { c, emit -> WebHost(c, emit) }
    }

    fun handle(call: PluginCall) = Plugins.unknownMethod(call)
}

@SuppressLint("SetJavaScriptEnabled")
private class WebHost(c: Context, emit: (String, String) -> Unit) : NimbusNativeView {
    private val web = WebView(c)
    private var last = ""

    override val view: View get() = web

    init {
        web.settings.javaScriptEnabled = true
        web.settings.domStorageEnabled = true
        web.webViewClient = object : WebViewClient() {
            override fun onPageFinished(v: WebView, url: String?) = emit("load", JSONObject().put("url", url ?: "").toString())
        }
    }

    override fun update(props: JSONObject) {
        val url = props.optString("url")
        val html = props.optString("html")
        if (url + html == last) return
        last = url + html
        if (url.isNotEmpty()) web.loadUrl(url) else web.loadDataWithBaseURL(null, html, "text/html", "utf-8", null)
    }
}
