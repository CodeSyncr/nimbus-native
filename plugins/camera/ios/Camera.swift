import PhotosUI
import UIKit

/// camera: the system's camera and photo picker, as a JPEG the right way up.
final class CameraPlugin: NSObject, UIImagePickerControllerDelegate, UINavigationControllerDelegate, PHPickerViewControllerDelegate {
    private static let shared = CameraPlugin()
    private var call: PluginCall?

    static func handle(_ call: PluginCall) { shared.handle(call) }

    private func handle(_ c: PluginCall) {
        guard let top = NimbusViewController.top else { return c.reject(Plugins.failed, "no screen to open it from") }
        switch c.method {
        case "take":
            guard UIImagePickerController.isSourceTypeAvailable(.camera) else { return c.reject(Plugins.unavailable, "this phone has no camera") }
            call = c
            let p = UIImagePickerController()
            p.sourceType = .camera
            p.delegate = self
            top.present(p, animated: true)
        case "pick":
            call = c
            var config = PHPickerConfiguration()
            config.filter = .images
            let p = PHPickerViewController(configuration: config)
            p.delegate = self
            top.present(p, animated: true)
        default:
            Plugins.unknownMethod(c)
        }
    }

    private func finish(_ image: UIImage?) {
        guard let c = call else { return }
        call = nil
        guard let image else { return c.reject(Plugins.cancelled, "no photo") }
        let longest = CGFloat(c.args["maxSize"] as? Int ?? 0)
        let q = c.args["quality"] as? Double ?? 0
        DispatchQueue.global(qos: .userInitiated).async {
            let limit = longest > 0 ? longest : 2000
            let scale = min(1, limit / max(image.size.width, image.size.height))
            let size = CGSize(width: (image.size.width * scale).rounded(), height: (image.size.height * scale).rounded())
            let f = UIGraphicsImageRendererFormat()
            f.scale = 1
            let out = UIGraphicsImageRenderer(size: size, format: f).jpegData(withCompressionQuality: q > 0 ? q : 0.85) { _ in
                image.draw(in: CGRect(origin: .zero, size: size))
            }
            c.resolve(["data": out.base64EncodedString(), "width": Int(size.width), "height": Int(size.height)])
        }
    }

    func imagePickerController(_ p: UIImagePickerController, didFinishPickingMediaWithInfo info: [UIImagePickerController.InfoKey: Any]) {
        p.dismiss(animated: true)
        finish(info[.originalImage] as? UIImage)
    }

    func imagePickerControllerDidCancel(_ p: UIImagePickerController) {
        p.dismiss(animated: true)
        finish(nil)
    }

    func picker(_ p: PHPickerViewController, didFinishPicking results: [PHPickerResult]) {
        p.dismiss(animated: true)
        guard let item = results.first?.itemProvider, item.canLoadObject(ofClass: UIImage.self) else { return finish(nil) }
        item.loadObject(ofClass: UIImage.self) { image, _ in
            DispatchQueue.main.async { self.finish(image as? UIImage) }
        }
    }
}
