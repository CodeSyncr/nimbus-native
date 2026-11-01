import UIKit

/// brightness: the screen's own setting, so the level before Set is kept for Reset.
enum BrightnessPlugin {
    private static var before: CGFloat?

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "get":
            call.resolve(["level": Double(UIScreen.main.brightness)])
        case "set":
            if before == nil { before = UIScreen.main.brightness }
            UIScreen.main.brightness = CGFloat(call.args["level"] as? Double ?? 1)
            call.resolve()
        case "reset":
            if let b = before { UIScreen.main.brightness = b }
            before = nil
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
