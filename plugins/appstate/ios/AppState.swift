import UIKit

/// appstate: the application's state, which works while backgrounded for as
/// long as the app runs.
enum AppStatePlugin {
    static func boot() {}

    static func handle(_ call: PluginCall) {
        guard call.method == "get" else { return Plugins.unknownMethod(call) }
        call.resolve(["active": UIApplication.shared.applicationState == .active])
    }
}
