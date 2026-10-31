import SafariServices
import UIKit

/// browser: Safari's in-app view.
enum BrowserPlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "open" else { return Plugins.unknownMethod(call) }
        guard let s = call.string("url"), let u = URL(string: s), u.scheme == "https" || u.scheme == "http" else {
            return call.reject(Plugins.failed, "that isn't a web address")
        }
        guard let top = NimbusViewController.top else { return call.reject(Plugins.failed, "no screen to open it from") }
        top.present(SFSafariViewController(url: u), animated: true) { call.resolve() }
    }
}
