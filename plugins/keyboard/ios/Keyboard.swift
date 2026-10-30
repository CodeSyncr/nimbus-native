import UIKit

/// keyboard: put it away.
enum KeyboardPlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "dismiss" else { return Plugins.unknownMethod(call) }
        UIApplication.shared.sendAction(#selector(UIResponder.resignFirstResponder), to: nil, from: nil, for: nil)
        call.resolve()
    }
}
