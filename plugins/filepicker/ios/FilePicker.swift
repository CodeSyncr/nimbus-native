import UIKit
import UniformTypeIdentifiers

/// filepicker: the system's document picker.
final class FilePickerPlugin: NSObject, UIDocumentPickerDelegate {
    private static let shared = FilePickerPlugin()
    private var call: PluginCall?

    static func handle(_ call: PluginCall) { shared.handle(call) }

    private func handle(_ c: PluginCall) {
        guard c.method == "pick" else { return Plugins.unknownMethod(c) }
        guard let top = NimbusViewController.top else { return c.reject(Plugins.failed, "no screen to open it from") }
        let wanted = (c.args["types"] as? [String] ?? []).compactMap { UTType(mimeType: $0) }
        call = c
        let p = UIDocumentPickerViewController(forOpeningContentTypes: wanted.isEmpty ? [.item] : wanted, asCopy: true)
        p.delegate = self
        top.present(p, animated: true)
    }

    func documentPicker(_ p: UIDocumentPickerViewController, didPickDocumentsAt urls: [URL]) {
        guard let c = call, let url = urls.first else { return }
        call = nil
        let most = (c.args["maxBytes"] as? Int64).flatMap { $0 > 0 ? $0 : nil } ?? 20 << 20
        DispatchQueue.global(qos: .userInitiated).async {
            guard let d = try? Data(contentsOf: url) else { return c.reject(Plugins.failed, "the file couldn't be read") }
            guard d.count <= most else { return c.reject(Plugins.failed, "the file is too large") }
            c.resolve(["name": url.lastPathComponent, "type": UTType(filenameExtension: url.pathExtension)?.preferredMIMEType ?? "", "data": d.base64EncodedString()])
        }
    }

    func documentPickerWasCancelled(_ p: UIDocumentPickerViewController) {
        call?.reject(Plugins.cancelled, "no file")
        call = nil
    }
}
