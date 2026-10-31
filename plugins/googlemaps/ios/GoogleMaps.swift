import GoogleMaps
import UIKit

/// googlemaps: the SDK starts with the key in Info.plist, and each map is a
/// native view (ui.Native).
enum GoogleMapsPlugin {
    static func boot() {
        if let key = Bundle.main.object(forInfoDictionaryKey: "GMSApiKey") as? String { GMSServices.provideAPIKey(key) }
        NativeViews.register("googlemaps") { emit in MapHost(emit) }
    }

    static func handle(_ call: PluginCall) { Plugins.unknownMethod(call) }
}

private func color(_ c: Int) -> UIColor {
    UIColor(red: CGFloat((c >> 24) & 255) / 255, green: CGFloat((c >> 16) & 255) / 255, blue: CGFloat((c >> 8) & 255) / 255, alpha: CGFloat(c & 255) / 255)
}

private final class MapHost: NSObject, NimbusNativeView, GMSMapViewDelegate {
    private let map: GMSMapView
    private let emit: (String, String) -> Void
    private var overlays: [GMSOverlay] = []
    private var lastCamera = ""
    private var lastOverlays = ""

    var view: UIView { map }

    init(_ emit: @escaping (String, String) -> Void) {
        self.emit = emit
        map = GMSMapView(options: GMSMapViewOptions())
        super.init()
        map.delegate = self
    }

    private func place(_ o: Any?) -> CLLocationCoordinate2D? {
        guard let d = o as? [String: Any], let lat = d["lat"] as? Double, let lng = d["lng"] as? Double else { return nil }
        return CLLocationCoordinate2D(latitude: lat, longitude: lng)
    }

    func update(_ p: [String: Any]) {
        let camera = "\(p["center"] ?? "")\(p["zoom"] ?? "")"
        if camera != lastCamera, let at = place(p["center"]) {
            let z = (p["zoom"] as? NSNumber)?.floatValue ?? 12
            if lastCamera.isEmpty { map.camera = GMSCameraPosition(target: at, zoom: z) } else { map.animate(to: GMSCameraPosition(target: at, zoom: z)) }
            lastCamera = camera
        }
        map.isMyLocationEnabled = p["myLocation"] as? Bool ?? false
        let shapes = "\(p["markers"] ?? "")\(p["lines"] ?? "")"
        guard shapes != lastOverlays else { return }
        lastOverlays = shapes
        overlays.forEach { $0.map = nil }
        overlays = []
        for m in p["markers"] as? [[String: Any]] ?? [] {
            guard let at = place(m["at"]) else { continue }
            let pin = GMSMarker(position: at)
            pin.title = m["title"] as? String
            pin.userData = m["id"]
            pin.map = map
            overlays.append(pin)
        }
        for l in p["lines"] as? [[String: Any]] ?? [] {
            let path = GMSMutablePath()
            for q in l["points"] as? [Any] ?? [] { if let at = place(q) { path.add(at) } }
            let line = GMSPolyline(path: path)
            line.strokeColor = color(l["color"] as? Int ?? 0)
            line.strokeWidth = CGFloat((l["width"] as? NSNumber)?.floatValue ?? 4)
            line.map = map
            overlays.append(line)
        }
    }

    private func json(_ d: [String: Any]) -> String {
        (try? JSONSerialization.data(withJSONObject: d)).map { String(decoding: $0, as: UTF8.self) } ?? "{}"
    }

    func mapView(_ m: GMSMapView, didTapAt c: CLLocationCoordinate2D) {
        emit("tap", json(["at": ["lat": c.latitude, "lng": c.longitude]]))
    }

    func mapView(_ m: GMSMapView, didTap marker: GMSMarker) -> Bool {
        emit("marker", json(["id": marker.userData as? String ?? ""]))
        return false
    }

    func mapView(_ m: GMSMapView, idleAt p: GMSCameraPosition) {
        emit("move", json(["at": ["lat": p.target.latitude, "lng": p.target.longitude], "zoom": p.zoom]))
    }
}
