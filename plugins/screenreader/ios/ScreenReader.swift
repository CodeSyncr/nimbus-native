import UIKit

/// screenreader: VoiceOver.
enum ScreenReaderPlugin {
    static func handle(_ call: PluginCall) {
        switch call.method {
        case "enabled":
            call.resolve(["enabled": UIAccessibility.isVoiceOverRunning])
        case "announce":
            UIAccessibility.post(notification: .announcement, argument: call.string("text") ?? "")
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
