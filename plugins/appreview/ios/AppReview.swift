import StoreKit
import UIKit

/// appreview: StoreKit's prompt.
enum AppReviewPlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "request" else { return Plugins.unknownMethod(call) }
        if let scene = NimbusViewController.current?.view.window?.windowScene { SKStoreReviewController.requestReview(in: scene) }
        call.resolve()
    }
}
