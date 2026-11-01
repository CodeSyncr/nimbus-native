import UIKit

/// screencapture: iOS can't block a screenshot, so a cover goes over the window while recording.
enum ScreenCapturePlugin {
    private static var prevent = false
    private static var count = 0
    private static var capture: NSObjectProtocol?
    private static var shots: NSObjectProtocol?
    private static var cover: UIView?

    private static func update() {
        let hide = prevent && UIScreen.main.isCaptured
        if hide, cover == nil, let w = NimbusViewController.current?.view.window {
            let v = UIView(frame: w.bounds)
            v.backgroundColor = .black
            v.autoresizingMask = [.flexibleWidth, .flexibleHeight]
            w.addSubview(v)
            cover = v
        } else if !hide {
            cover?.removeFromSuperview()
            cover = nil
        }
    }

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "set":
            prevent = call.args["prevent"] as? Bool ?? false
            if capture == nil {
                capture = NotificationCenter.default.addObserver(forName: UIScreen.capturedDidChangeNotification, object: nil, queue: .main) { _ in update() }
            }
            update()
            call.resolve()
        case "start":
            count = 0
            if shots == nil {
                shots = NotificationCenter.default.addObserver(forName: UIApplication.userDidTakeScreenshotNotification, object: nil, queue: .main) { _ in count += 1 }
            }
            call.resolve()
        case "read":
            call.resolve(["count": count])
        case "stop":
            if let s = shots { NotificationCenter.default.removeObserver(s) }
            shots = nil
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
