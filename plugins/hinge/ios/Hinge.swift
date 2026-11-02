import UIKit

/// hinge: no iPhone or iPad folds, so the probe view never reports one.
enum HingePlugin {
    static func boot() {
        NativeViews.register("hinge") { _ in NoFold() }
    }

    static func handle(_ call: PluginCall) { Plugins.unknownMethod(call) }
}

private final class NoFold: NimbusNativeView {
    let view = UIView()
    func update(_ p: [String: Any]) {}
}
