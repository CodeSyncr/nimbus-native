import UIKit
import WebKit

/// webview: WKWebView as a native view (ui.Native).
enum WebViewPlugin {
    static func boot() { NativeViews.register("webview") { emit in WebHost(emit) } }
    static func handle(_ call: PluginCall) { Plugins.unknownMethod(call) }
}

private final class WebHost: NSObject, NimbusNativeView, WKNavigationDelegate {
    private let web = WKWebView()
    private let emit: (String, String) -> Void
    private var last = ""

    var view: UIView { web }

    init(_ emit: @escaping (String, String) -> Void) {
        self.emit = emit
        super.init()
        web.navigationDelegate = self
    }

    func update(_ p: [String: Any]) {
        let url = p["url"] as? String ?? "", html = p["html"] as? String ?? ""
        guard url + html != last else { return }
        last = url + html
        if let u = URL(string: url), !url.isEmpty { web.load(URLRequest(url: u)) } else { web.loadHTMLString(html, baseURL: nil) }
    }

    func webView(_ w: WKWebView, didFinish n: WKNavigation!) {
        let d = (try? JSONSerialization.data(withJSONObject: ["url": w.url?.absoluteString ?? ""])) ?? Data()
        emit("load", String(decoding: d, as: UTF8.self))
    }
}
