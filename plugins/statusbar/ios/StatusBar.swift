import UIKit

/// statusbar: through the root view controller, which owns the bar.
enum StatusBarPlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "set" else { return Plugins.unknownMethod(call) }
        if let s = call.string("style") {
            NimbusViewController.statusBarStyle = s == "light" ? .lightContent : s == "dark" ? .darkContent : .default
        }
        if let h = call.args["hidden"] as? Bool { NimbusViewController.statusBarHidden = h }
        NimbusViewController.current?.setNeedsStatusBarAppearanceUpdate()
        call.resolve()
    }
}
