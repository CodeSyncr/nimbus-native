import UIKit

/// clipboard: text on the pasteboard.
enum ClipboardPlugin {
    static func handle(_ call: PluginCall) {
        switch call.method {
        case "set":
            UIPasteboard.general.string = call.string("text") ?? ""
            call.resolve()
        case "get":
            call.resolve(["text": UIPasteboard.general.string ?? ""])
        default:
            Plugins.unknownMethod(call)
        }
    }
}
