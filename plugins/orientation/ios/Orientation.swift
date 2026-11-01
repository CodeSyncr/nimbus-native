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
            // Widening the mask rotates nothing, so unlocking asks for how the phone is held.
            let target = m == "portrait" || m == "landscape" ? mask : held()
            vc?.view.window?.windowScene?.requestGeometryUpdate(.iOS(interfaceOrientations: target))
        } else {
            UIViewController.attemptRotationToDeviceOrientation()
        }
        call.resolve()
    }

    /// The way up the phone is, or portrait when it can't say. Screen and device landscape are opposites.
    private static func held() -> UIInterfaceOrientationMask {
        UIDevice.current.beginGeneratingDeviceOrientationNotifications()
        defer { UIDevice.current.endGeneratingDeviceOrientationNotifications() }
        switch UIDevice.current.orientation {
        case .landscapeLeft: return .landscapeRight
        case .landscapeRight: return .landscapeLeft
        default: return .portrait
        }
    }
}
