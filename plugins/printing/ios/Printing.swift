import UIKit

/// printing: the system's print sheet.
enum PrintingPlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "print" else { return Plugins.unknownMethod(call) }
        let p = UIPrintInteractionController.shared
        let info = UIPrintInfo.printInfo()
        info.jobName = call.string("name") ?? "Document"
        p.printInfo = info
        p.printFormatter = nil
        p.printingItem = nil
        if let b = call.string("pdf"), let d = Data(base64Encoded: b) {
            p.printingItem = d
        } else if let h = call.string("html") {
            p.printFormatter = UIMarkupTextPrintFormatter(markupText: h)
        } else {
            return call.reject(Plugins.failed, "nothing to print")
        }
        p.present(animated: true) { _, done, e in
            if let e { call.reject(Plugins.failed, e.localizedDescription) }
            else if done { call.resolve() }
            else { call.reject(Plugins.cancelled, "they backed out") }
        }
    }
}
