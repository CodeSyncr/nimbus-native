import UIKit

/// splashscreen: a view of the launch color over the app until Hide.
enum SplashPlugin {
    private static var cover: UIView?

    static func boot() {
        DispatchQueue.main.async {
            guard let root = NimbusViewController.current?.view else { return }
            let v = UIView(frame: root.bounds)
            v.autoresizingMask = [.flexibleWidth, .flexibleHeight]
            v.backgroundColor = UIColor(named: "LaunchBackground") ?? .systemBackground
            root.addSubview(v)
            cover = v
            DispatchQueue.main.asyncAfter(deadline: .now() + 10) { hide() }
        }
    }

    private static func hide() {
        guard let v = cover else { return }
        cover = nil
        UIView.animate(withDuration: 0.25, animations: { v.alpha = 0 }, completion: { _ in v.removeFromSuperview() })
    }

    static func handle(_ call: PluginCall) {
        guard call.method == "hide" else { return Plugins.unknownMethod(call) }
        hide()
        call.resolve()
    }
}
