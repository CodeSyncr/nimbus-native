import UIKit
import UniformTypeIdentifiers

/// The Share Extension nimbus native plugin add receive-share puts in the
/// Xcode project. It copies what was shared into the App Group the app
/// shares with it, says so, and closes. The app takes it from there
/// (package receiveshare). Changes here are overwritten by plugin add.
final class ShareViewController: UIViewController {
    private let label = UILabel()

    /// group.<the app's bundle ID>: this extension's ID is that, plus ".ShareExtension".
    private var group: URL? {
        let id = (Bundle.main.bundleIdentifier ?? "").replacingOccurrences(of: ".ShareExtension", with: "")
        return FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: "group." + id)?
            .appendingPathComponent("nimbus-shared", isDirectory: true)
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
        label.font = .preferredFont(forTextStyle: .headline)
        label.textAlignment = .center
        label.numberOfLines = 0
        label.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(label)
        NSLayoutConstraint.activate([
            label.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            label.centerYAnchor.constraint(equalTo: view.centerYAnchor),
            label.widthAnchor.constraint(lessThanOrEqualTo: view.widthAnchor, constant: -48),
        ])
        let name = Bundle.main.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String ?? "the app"
        label.text = "Sending to \(name)…"
        Task { await save(name) }
    }

    private func save(_ name: String) async {
        guard let root = group else { return done("This app can't receive shares yet: its App Group is missing.") }
        let dir = root.appendingPathComponent(UUID().uuidString, isDirectory: true)
        var text = "", link = "", subject = ""
        var files: [[String: Any]] = []
        do {
            try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            for item in extensionContext?.inputItems as? [NSExtensionItem] ?? [] {
                if subject.isEmpty, let t = item.attributedContentText?.string { subject = t }
                for p in item.attachments ?? [] {
                    if p.hasItemConformingToTypeIdentifier(UTType.url.identifier), !p.hasItemConformingToTypeIdentifier(UTType.fileURL.identifier),
                       let u = try? await p.loadItem(forTypeIdentifier: UTType.url.identifier) as? URL {
                        link = u.absoluteString
                        if text.isEmpty { text = link }
                    } else if p.hasItemConformingToTypeIdentifier(UTType.plainText.identifier), !p.hasItemConformingToTypeIdentifier(UTType.fileURL.identifier),
                              let t = try? await p.loadItem(forTypeIdentifier: UTType.plainText.identifier) as? String {
                        text = t
                    } else if let type = p.registeredTypeIdentifiers.first, let f = await copy(p, type, into: dir) {
                        files.append(f)
                    }
                }
            }
            if link.isEmpty, let m = text.range(of: #"https?://\S+"#, options: .regularExpression) { link = String(text[m]) }
            if subject == text { subject = "" }
            let share: [String: Any] = ["text": text, "url": link, "subject": subject, "files": files,
                                        "time": Date().timeIntervalSince1970]
            // Written last, so the app never reads half a share.
            try JSONSerialization.data(withJSONObject: share).write(to: dir.appendingPathComponent("share.json"), options: .atomic)
            done("Sent to \(name)")
        } catch {
            try? FileManager.default.removeItem(at: dir)
            done("Couldn't send that: \(error.localizedDescription)")
        }
    }

    private func copy(_ p: NSItemProvider, _ type: String, into dir: URL) async -> [String: Any]? {
        await withCheckedContinuation { (k: CheckedContinuation<[String: Any]?, Never>) in
            _ = p.loadFileRepresentation(forTypeIdentifier: type) { url, _ in
                guard let url else { return k.resume(returning: nil) }
                let file = UUID().uuidString
                do {
                    try FileManager.default.copyItem(at: url, to: dir.appendingPathComponent(file))
                    let mime = UTType(type)?.preferredMIMEType ?? "application/octet-stream"
                    k.resume(returning: ["name": p.suggestedName.map { n in url.pathExtension.isEmpty ? n : n + "." + url.pathExtension } ?? url.lastPathComponent,
                                         "type": mime, "file": file])
                } catch {
                    k.resume(returning: nil)
                }
            }
        }
    }

    @MainActor private func done(_ message: String) {
        label.text = message
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {
            self.extensionContext?.completeRequest(returningItems: nil)
        }
    }
}
