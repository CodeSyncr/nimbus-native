import AVFoundation
import PhotosUI
import UIKit
import UniformTypeIdentifiers

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
        case "pickMany", "pickVideo":
            call = c
            var config = PHPickerConfiguration()
            config.filter = c.method == "pickVideo" ? .videos : .images
            config.selectionLimit = c.method == "pickVideo" ? 1 : c.args["limit"] as? Int ?? 0
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
        DispatchQueue.global(qos: .userInitiated).async { c.resolve(self.photo(image, c)) }
    }

    private func photo(_ image: UIImage, _ c: PluginCall) -> [String: Any] {
        let longest = CGFloat(c.args["maxSize"] as? Int ?? 0)
        let q = c.args["quality"] as? Double ?? 0
        let limit = longest > 0 ? longest : 2000
        let scale = min(1, limit / max(image.size.width, image.size.height))
        let size = CGSize(width: (image.size.width * scale).rounded(), height: (image.size.height * scale).rounded())
        let f = UIGraphicsImageRendererFormat()
        f.scale = 1
        let out = UIGraphicsImageRenderer(size: size, format: f).jpegData(withCompressionQuality: q > 0 ? q : 0.85) { _ in
            image.draw(in: CGRect(origin: .zero, size: size))
        }
        return ["data": out.base64EncodedString(), "width": Int(size.width), "height": Int(size.height)]
    }

    private func many(_ results: [PHPickerResult], _ c: PluginCall) {
        var images = [UIImage?](repeating: nil, count: results.count)
        let group = DispatchGroup()
        for (i, r) in results.enumerated() where r.itemProvider.canLoadObject(ofClass: UIImage.self) {
            group.enter()
            r.itemProvider.loadObject(ofClass: UIImage.self) { image, _ in
                DispatchQueue.main.async {
                    images[i] = image as? UIImage
                    group.leave()
                }
            }
        }
        group.notify(queue: .global(qos: .userInitiated)) {
            c.resolve(images.compactMap { $0 }.map { self.photo($0, c) })
        }
    }

    private func video(_ item: NSItemProvider, _ c: PluginCall) {
        item.loadFileRepresentation(forTypeIdentifier: UTType.movie.identifier) { url, e in
            guard let url else { return c.reject(Plugins.failed, e?.localizedDescription ?? "the video couldn't be read") }
            let limit = (c.args["maxBytes"] as? Int) ?? 50 << 20
            let size = (try? url.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
            if size > limit { return c.reject(Plugins.failed, "the video is larger than MaxBytes") }
            guard let d = try? Data(contentsOf: url) else { return c.reject(Plugins.failed, "the video couldn't be read") }
            let seconds = AVURLAsset(url: url).duration.seconds
            let type = UTType(filenameExtension: url.pathExtension)?.preferredMIMEType ?? "video/mp4"
            c.resolve(["data": d.base64EncodedString(), "type": type, "seconds": seconds.isFinite ? seconds : 0])
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
        if let c = call, c.method != "pick" {
            call = nil
            if results.isEmpty { return c.reject(Plugins.cancelled, "nothing chosen") }
            return c.method == "pickMany" ? many(results, c) : video(results[0].itemProvider, c)
        }
        guard let item = results.first?.itemProvider, item.canLoadObject(ofClass: UIImage.self) else { return finish(nil) }
        item.loadObject(ofClass: UIImage.self) { image, _ in
            DispatchQueue.main.async { self.finish(image as? UIImage) }
        }
    }
}
