import AVFoundation
import UIKit

/// barcode: a full-screen camera that closes on the first code.
enum BarcodePlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "scan" else { return Plugins.unknownMethod(call) }
        guard AVCaptureDevice.default(for: .video) != nil else { return call.reject(Plugins.unavailable, "this phone has no camera") }
        AVCaptureDevice.requestAccess(for: .video) { ok in
            DispatchQueue.main.async {
                guard ok else { return call.reject(Plugins.denied, "the camera is not allowed") }
                guard let top = NimbusViewController.top else { return call.reject(Plugins.failed, "no screen to open it from") }
                let v = ScannerController(call)
                v.modalPresentationStyle = .fullScreen
                top.present(v, animated: true)
            }
        }
    }
}

private final class ScannerController: UIViewController, AVCaptureMetadataOutputObjectsDelegate {
    private let call: PluginCall
    private let session = AVCaptureSession()
    private let names: [AVMetadataObject.ObjectType: String] = [.qr: "qr", .ean13: "ean13", .ean8: "ean8", .code128: "code128", .code39: "code39", .upce: "upce", .pdf417: "pdf417", .dataMatrix: "datamatrix"]

    init(_ call: PluginCall) {
        self.call = call
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) { fatalError("made in code") }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .black
        let out = AVCaptureMetadataOutput()
        guard let cam = AVCaptureDevice.default(for: .video), let input = try? AVCaptureDeviceInput(device: cam),
              session.canAddInput(input), session.canAddOutput(out) else { return close { self.call.reject(Plugins.failed, "the camera couldn't start") } }
        session.addInput(input)
        session.addOutput(out)
        out.setMetadataObjectsDelegate(self, queue: .main)
        out.metadataObjectTypes = Array(names.keys)
        let preview = AVCaptureVideoPreviewLayer(session: session)
        preview.frame = view.bounds
        preview.videoGravity = .resizeAspectFill
        view.layer.addSublayer(preview)
        let cancel = UIButton(type: .system)
        cancel.setTitle("Cancel", for: .normal)
        cancel.setTitleColor(.white, for: .normal)
        cancel.frame = CGRect(x: 20, y: view.safeAreaInsets.top + 60, width: 80, height: 44)
        cancel.addTarget(self, action: #selector(cancelled), for: .touchUpInside)
        view.addSubview(cancel)
        DispatchQueue.global(qos: .userInitiated).async { self.session.startRunning() }
    }

    private func close(_ then: @escaping () -> Void) {
        session.stopRunning()
        dismiss(animated: true, completion: then)
    }

    @objc private func cancelled() { close { self.call.reject(Plugins.cancelled, "no code") } }

    func metadataOutput(_ o: AVCaptureMetadataOutput, didOutput objects: [AVMetadataObject], from c: AVCaptureConnection) {
        guard let code = objects.first as? AVMetadataMachineReadableCodeObject, let value = code.stringValue else { return }
        session.stopRunning()
        close { self.call.resolve(["value": value, "format": self.names[code.type] ?? "other"]) }
    }
}
