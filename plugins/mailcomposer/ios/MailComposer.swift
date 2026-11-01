import MessageUI
import UIKit

/// mailcomposer: the system's mail sheet.
final class MailComposerPlugin: NSObject, MFMailComposeViewControllerDelegate {
    private static let shared = MailComposerPlugin()
    private var call: PluginCall?

    static func handle(_ c: PluginCall) {
        switch c.method {
        case "available":
            c.resolve(["available": MFMailComposeViewController.canSendMail()])
        case "compose":
            guard MFMailComposeViewController.canSendMail() else { return c.reject(Plugins.unavailable, "no mail account on this phone") }
            guard let top = NimbusViewController.top else { return c.reject(Plugins.failed, "no screen to open it from") }
            let v = MFMailComposeViewController()
            v.setToRecipients(c.args["to"] as? [String])
            v.setCcRecipients(c.args["cc"] as? [String])
            v.setBccRecipients(c.args["bcc"] as? [String])
            v.setSubject(c.string("subject") ?? "")
            v.setMessageBody(c.string("body") ?? "", isHTML: c.args["html"] as? Bool ?? false)
            for f in c.args["files"] as? [[String: Any]] ?? [] {
                guard let name = f["name"] as? String, let b = f["data"] as? String, let d = Data(base64Encoded: b) else { continue }
                v.addAttachmentData(d, mimeType: f["type"] as? String ?? "application/octet-stream", fileName: name)
            }
            v.mailComposeDelegate = shared
            shared.call = c
            top.present(v, animated: true)
        default:
            Plugins.unknownMethod(c)
        }
    }

    func mailComposeController(_ v: MFMailComposeViewController, didFinishWith r: MFMailComposeResult, error: Error?) {
        v.dismiss(animated: true)
        if r == .failed {
            call?.reject(Plugins.failed, error?.localizedDescription ?? "it couldn't be sent")
        } else {
            call?.resolve(["status": r == .sent ? "sent" : r == .saved ? "saved" : "cancelled"])
        }
        call = nil
    }
}
