import Foundation

/// receiveshare: the Share Extension leaves each share in the App Group, a
/// folder with share.json and its files. This reads them and clears them.
enum ReceiveSharePlugin {
    private static var inbox: URL? {
        FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: "group." + (Bundle.main.bundleIdentifier ?? ""))?
            .appendingPathComponent("nimbus-shared", isDirectory: true)
    }

    static func handle(_ call: PluginCall) {
        guard call.method == "take" else { return Plugins.unknownMethod(call) }
        let most = (call.args["maxBytes"] as? NSNumber)?.int64Value ?? 0
        DispatchQueue.global(qos: .userInitiated).async {
            guard let root = inbox else { return call.reject(Plugins.unavailable, "the App Group is missing: add receive-share again") }
            let fm = FileManager.default
            var shares: [[String: Any]] = []
            for dir in (try? fm.contentsOfDirectory(at: root, includingPropertiesForKeys: nil)) ?? [] {
                guard let b = try? Data(contentsOf: dir.appendingPathComponent("share.json")),
                      var s = (try? JSONSerialization.jsonObject(with: b)) as? [String: Any] else { continue } // still being written
                s["files"] = (s["files"] as? [[String: Any]] ?? []).map { f -> [String: Any] in
                    var out = f
                    let on = dir.appendingPathComponent(f["file"] as? String ?? "")
                    out["file"] = nil
                    let size = ((try? fm.attributesOfItem(atPath: on.path))?[.size] as? NSNumber)?.int64Value ?? 0
                    out["size"] = size
                    if size <= (most > 0 ? most : 20 << 20), let d = try? Data(contentsOf: on) { out["data"] = d.base64EncodedString() }
                    return out
                }
                shares.append(s)
                try? fm.removeItem(at: dir)
            }
            shares.sort { ($0["time"] as? Double ?? 0) < ($1["time"] as? Double ?? 0) }
            call.resolve(shares)
        }
    }
}
