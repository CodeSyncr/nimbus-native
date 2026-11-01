import AVFoundation
import UIKit

/// cameraview: a capture session behind a native view (ui.Native).
final class CameraViewPlugin: NSObject {
    fileprivate static weak var current: CameraHost?

    static func boot() { NativeViews.register("cameraview") { emit in CameraHost(emit) } }

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "request":
            AVCaptureDevice.requestAccess(for: .video) { ok in
                call.resolve(["granted": ok])
                if ok { DispatchQueue.main.async { current?.start() } }
            }
        case "take":
            guard let h = current, h.running else { return call.reject(Plugins.unavailable, "no camera is showing") }
            h.take(call)
        default:
            Plugins.unknownMethod(call)
        }
    }
}

private final class PreviewView: UIView {
    override class var layerClass: AnyClass { AVCaptureVideoPreviewLayer.self }
    var preview: AVCaptureVideoPreviewLayer { layer as! AVCaptureVideoPreviewLayer }
    var onWindow: ((Bool) -> Void)?

    override func didMoveToWindow() {
        super.didMoveToWindow()
        onWindow?(window != nil)
    }
}

private final class CameraHost: NSObject, NimbusNativeView, AVCapturePhotoCaptureDelegate {
    private let box = PreviewView()
    private let session = AVCaptureSession()
    private let output = AVCapturePhotoOutput()
    private let queue = DispatchQueue(label: "nimbus.camera")
    private let emit: (String, String) -> Void
    private var front = false
    private var torch = false
    private var input: AVCaptureDeviceInput?
    private var pending: PluginCall?
    fileprivate var running = false

    var view: UIView { box }

    init(_ emit: @escaping (String, String) -> Void) {
        self.emit = emit
        super.init()
        box.backgroundColor = .black
        box.preview.session = session
        box.preview.videoGravity = .resizeAspectFill
        box.onWindow = { [weak self] on in
            if on { self?.start() } else { self?.stop() }
        }
    }

    func update(_ p: [String: Any]) {
        let f = p["front"] as? Bool ?? false
        torch = p["torch"] as? Bool ?? false
        if f != front {
            front = f
            if running { queue.async { self.attach() } } else { start() }
        }
        queue.async { self.applyTorch() }
        CameraViewPlugin.current = self
    }

    private func fail(_ code: String) { DispatchQueue.main.async { self.emit("error", "{\"code\":\"\(code)\"}") } }

    fileprivate func start() {
        CameraViewPlugin.current = self
        guard box.window != nil else { return }
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized: break
        case .notDetermined: return // Request starts it
        default: return fail(Plugins.denied)
        }
        queue.async {
            guard self.attach() else { return }
            if !self.session.outputs.contains(self.output), self.session.canAddOutput(self.output) { self.session.addOutput(self.output) }
            self.session.startRunning()
            self.running = true
            self.applyTorch()
            DispatchQueue.main.async { self.emit("ready", "{}") }
        }
    }

    private func stop() {
        running = false
        queue.async { self.session.stopRunning() }
    }

    @discardableResult private func attach() -> Bool {
        let d = AVCaptureDevice.default(.builtInWideAngleCamera, for: .video, position: front ? .front : .back)
        guard let d, let i = try? AVCaptureDeviceInput(device: d) else {
            fail(Plugins.unavailable)
            return false
        }
        session.beginConfiguration()
        if let old = input { session.removeInput(old) }
        session.sessionPreset = .photo
        if session.canAddInput(i) { session.addInput(i) }
        input = i
        session.commitConfiguration()
        return true
    }

    private func applyTorch() {
        guard let d = input?.device, d.hasTorch, (try? d.lockForConfiguration()) != nil else { return }
        d.torchMode = torch ? .on : .off
        d.unlockForConfiguration()
    }

    fileprivate func take(_ call: PluginCall) {
        pending = call
        queue.async {
            self.output.connection(with: .video)?.videoOrientation = .portrait
            self.output.capturePhoto(with: AVCapturePhotoSettings(), delegate: self)
        }
    }

    func photoOutput(_ o: AVCapturePhotoOutput, didFinishProcessingPhoto p: AVCapturePhoto, error: Error?) {
        guard let call = pending else { return }
        pending = nil
        guard let d = p.fileDataRepresentation(), let image = UIImage(data: d) else {
            return call.reject(Plugins.failed, error?.localizedDescription ?? "no photo came back")
        }
        let max = CGFloat(call.args["maxSize"] as? Int ?? 0)
        let quality = (call.args["quality"] as? Double).flatMap { $0 > 0 ? $0 : nil } ?? 0.85
        let scale = max > 0 ? min(1, max / Swift.max(image.size.width, image.size.height)) : 1
        let size = CGSize(width: (image.size.width * scale).rounded(), height: (image.size.height * scale).rounded())
        let format = UIGraphicsImageRendererFormat()
        format.scale = 1
        let upright = UIGraphicsImageRenderer(size: size, format: format).image { _ in image.draw(in: CGRect(origin: .zero, size: size)) }
        guard let jpeg = upright.jpegData(compressionQuality: quality) else { return call.reject(Plugins.failed, "no photo came back") }
        call.resolve(["data": jpeg.base64EncodedString(), "width": Int(size.width), "height": Int(size.height)])
    }
}
