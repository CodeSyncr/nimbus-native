import MessageUI
import UIKit

/// sms: the system's message sheet.
final class SmsPlugin: NSObject, MFMessageComposeViewControllerDelegate {
    private static let shared = SmsPlugin()
    private var call: PluginCall?

    static func handle(_ c: PluginCall) {
        switch c.method {
        case "available":
            c.resolve(["available": MFMessageComposeViewController.canSendText()])
        case "compose":
            guard MFMessageComposeViewController.canSendText() else { return c.reject(Plugins.unavailable, "this phone can't send texts") }
            guard let top = NimbusViewController.top else { return c.reject(Plugins.failed, "no screen to open it from") }
            let v = MFMessageComposeViewController()
            v.recipients = c.args["to"] as? [String]
            v.body = c.string("body")
            v.messageComposeDelegate = shared
            shared.call = c
            top.present(v, animated: true)
        default:
            Plugins.unknownMethod(c)
        }
    }

    func messageComposeViewController(_ v: MFMessageComposeViewController, didFinishWith r: MessageComposeResult) {
        v.dismiss(animated: true)
        if r == .failed {
            call?.reject(Plugins.failed, "it couldn't be sent")
        } else {
            call?.resolve(["status": r == .sent ? "sent" : "cancelled"])
        }
        call = nil
    }
}
