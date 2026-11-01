import Photos
import UIKit

/// medialibrary: Photos.
enum MediaLibraryPlugin {
    static func handle(_ call: PluginCall) {
        switch call.method {
        case "request":
            PHPhotoLibrary.requestAuthorization(for: .readWrite) { s in
                call.resolve(["granted": s == .authorized || s == .limited])
            }
        case "save": save(call)
        case "list": list(call)
        case "read": read(call)
        default: Plugins.unknownMethod(call)
        }
    }

    private static func save(_ call: PluginCall) {
        guard let b = call.string("data"), let d = Data(base64Encoded: b) else { return call.reject(Plugins.failed, "nothing to save") }
        let video = (call.string("type") ?? "").hasPrefix("video/")
        var id: String?
        PHPhotoLibrary.shared().performChanges({
            let r = PHAssetCreationRequest.forAsset()
            r.addResource(with: video ? .video : .photo, data: d, options: nil)
            id = r.placeholderForCreatedAsset?.localIdentifier
        }) { ok, e in
            if ok { call.resolve(["id": id ?? ""]) }
            else if PHPhotoLibrary.authorizationStatus(for: .addOnly) == .denied { call.reject(Plugins.denied, "they said no") }
            else { call.reject(Plugins.failed, e?.localizedDescription ?? "it couldn't be saved") }
        }
    }

    private static func list(_ call: PluginCall) {
        let o = PHFetchOptions()
        o.sortDescriptors = [NSSortDescriptor(key: "creationDate", ascending: false)]
        o.fetchLimit = call.args["limit"] as? Int ?? 50
        var out: [[String: Any]] = []
        PHAsset.fetchAssets(with: o).enumerateObjects { a, _, _ in
            out.append(["id": a.localIdentifier, "video": a.mediaType == .video, "width": a.pixelWidth, "height": a.pixelHeight,
                        "time": a.creationDate?.timeIntervalSince1970 ?? 0])
        }
        call.resolve(out)
    }

    private static func read(_ call: PluginCall) {
        guard let a = PHAsset.fetchAssets(withLocalIdentifiers: [call.string("id") ?? ""], options: nil).firstObject else {
            return call.reject(Plugins.notFound, "no such item")
        }
        let max = call.args["maxSize"] as? Int ?? 0
        let o = PHImageRequestOptions()
        o.isNetworkAccessAllowed = true
        o.deliveryMode = .highQualityFormat
        o.isSynchronous = false
        let size = max > 0 ? CGSize(width: max, height: max) : PHImageManagerMaximumSize
        PHImageManager.default().requestImage(for: a, targetSize: size, contentMode: .aspectFit, options: o) { image, _ in
            guard let d = image?.jpegData(compressionQuality: 0.85) else { return call.reject(Plugins.failed, "it couldn't be read") }
            call.resolve(["data": d.base64EncodedString()])
        }
    }
}
