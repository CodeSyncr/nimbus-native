import Photos
import PhotosUI
import UIKit
import UniformTypeIdentifiers

/// livephoto: the photo picker with only Live Photos, which hands each one
/// over as a PHLivePhoto, kept here by ID; and PHLivePhotoView to play them.
enum LivePhotoPlugin {
    private static var photos: [String: PHLivePhoto] = [:] // main thread only
    private static var picking: Picker?

    static func boot() {
        NativeViews.register("livephoto") { _ in LivePhotoHost() }
    }

    static func photo(_ id: String) -> PHLivePhoto? { photos[id] }

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "pick": pick(call)
        case "files": files(call)
        case "forget":
            photos[call.string("id") ?? ""] = nil
            call.resolve()
        default: Plugins.unknownMethod(call)
        }
    }

    private final class Picker: NSObject, PHPickerViewControllerDelegate {
        let call: PluginCall
        init(_ call: PluginCall) { self.call = call }

        func picker(_ p: PHPickerViewController, didFinishPicking results: [PHPickerResult]) {
            p.dismiss(animated: true)
            LivePhotoPlugin.picking = nil
            guard let item = results.first?.itemProvider, item.canLoadObject(ofClass: PHLivePhoto.self) else {
                return call.reject(results.isEmpty ? Plugins.cancelled : Plugins.failed, results.isEmpty ? "they backed out" : "that isn't a Live Photo")
            }
            item.loadObject(ofClass: PHLivePhoto.self) { object, error in
                DispatchQueue.main.async {
                    guard let live = object as? PHLivePhoto else {
                        return self.call.reject(Plugins.failed, error?.localizedDescription ?? "the Live Photo didn't load")
                    }
                    let id = UUID().uuidString
                    LivePhotoPlugin.photos[id] = live
                    self.call.resolve(["id": id, "width": Int(live.size.width), "height": Int(live.size.height)])
                }
            }
        }
    }

    private static func pick(_ call: PluginCall) {
        guard let top = NimbusViewController.top else { return call.reject(Plugins.failed, "no screen to open it from") }
        var c = PHPickerConfiguration()
        c.filter = .livePhotos
        c.selectionLimit = 1
        c.preferredAssetRepresentationMode = .current
        let p = PHPickerViewController(configuration: c)
        let delegate = Picker(call)
        p.delegate = delegate
        picking = delegate
        top.present(p, animated: true)
    }

    /// Writes both halves out through PHAssetResourceManager and reads them back.
    private static func files(_ call: PluginCall) {
        guard let live = photos[call.string("id") ?? ""] else { return call.reject(Plugins.notFound, "no Live Photo with that ID: pick it again") }
        let resources = PHAssetResource.assetResources(for: live)
        var out: [String: Any] = [:]
        let group = DispatchGroup()
        var failure: Error?
        for r in resources where r.type == .photo || r.type == .pairedVideo {
            let url = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
            group.enter()
            PHAssetResourceManager.default().writeData(for: r, toFile: url, options: nil) { error in
                if let error { failure = error } else if let d = try? Data(contentsOf: url) {
                    let type = UTType(r.uniformTypeIdentifier)?.preferredMIMEType ?? "application/octet-stream"
                    DispatchQueue.main.async {
                        if r.type == .photo { out["photo"] = d.base64EncodedString(); out["photoType"] = type }
                        else { out["video"] = d.base64EncodedString(); out["videoType"] = type }
                    }
                }
                try? FileManager.default.removeItem(at: url)
                group.leave()
            }
        }
        group.notify(queue: .main) {
            if let failure { return call.reject(Plugins.failed, failure.localizedDescription) }
            call.resolve(out)
        }
    }
}

private final class LivePhotoHost: NimbusNativeView {
    private let live = PHLivePhotoView()
    private var id = ""
    private var play = 0
    var view: UIView { live }

    init() { live.clipsToBounds = true }

    func update(_ p: [String: Any]) {
        live.contentMode = (p["fill"] as? Bool ?? false) ? .scaleAspectFill : .scaleAspectFit
        live.isMuted = p["muted"] as? Bool ?? false
        let next = p["id"] as? String ?? ""
        if next != id {
            id = next
            live.livePhoto = LivePhotoPlugin.photo(next)
        }
        let n = (p["play"] as? NSNumber)?.intValue ?? 0
        if n != play {
            play = n
            if n > 0 { live.startPlayback(with: .full) }
        }
    }
}
