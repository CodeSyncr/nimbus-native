import UIKit

/// keepawake: the idle timer.
enum KeepAwakePlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "set" else { return Plugins.unknownMethod(call) }
        UIApplication.shared.isIdleTimerDisabled = call.args["on"] as? Bool ?? false
        call.resolve()
    }
}
