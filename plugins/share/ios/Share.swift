import UIKit

/// share: the activity sheet.
enum SharePlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "share" else { return Plugins.unknownMethod(call) }
        var items: [Any] = []
        if let t = call.string("text") { items.append(t) }
        if let s = call.string("url"), let u = URL(string: s) { items.append(u) }
        guard !items.isEmpty else { return call.reject(Plugins.failed, "nothing to share") }
        guard let root = NimbusViewController.current else { return call.reject(Plugins.failed, "no screen to share from") }
        let sheet = UIActivityViewController(activityItems: items, applicationActivities: nil)
        sheet.popoverPresentationController?.sourceView = root.view
        sheet.popoverPresentationController?.sourceRect = CGRect(x: root.view.bounds.midX, y: root.view.bounds.midY, width: 0, height: 0)
        var top: UIViewController = root
        while let p = top.presentedViewController { top = p }
        top.present(sheet, animated: true) { call.resolve() }
    }
}
