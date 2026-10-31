import UIKit

/// orientation: through the root view controller, which owns the mask.
enum OrientationPlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "set" else { return Plugins.unknownMethod(call) }
        let m = call.string("mode")
        let mask: UIInterfaceOrientationMask = m == "portrait" ? .portrait : m == "landscape" ? .landscape : .all
        NimbusViewController.orientations = mask
        let vc = NimbusViewController.current
        if #available(iOS 16, *) {
            vc?.setNeedsUpdateOfSupportedInterfaceOrientations()
            vc?.view.window?.windowScene?.requestGeometryUpdate(.iOS(interfaceOrientations: mask))
        } else {
            UIViewController.attemptRotationToDeviceOrientation()
        }
        call.resolve()
    }
}
