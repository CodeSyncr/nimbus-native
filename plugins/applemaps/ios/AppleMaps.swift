import MapKit
import UIKit

/// applemaps: each map is a native view (ui.Native) around MKMapView. MapKit
/// needs no key.
enum AppleMapsPlugin {
    static func boot() {
        NativeViews.register("applemaps") { emit in AppleMapHost(emit) }
    }

    static func handle(_ call: PluginCall) { Plugins.unknownMethod(call) }
}

private func color(_ c: Int) -> UIColor {
    UIColor(red: CGFloat((c >> 24) & 255) / 255, green: CGFloat((c >> 16) & 255) / 255, blue: CGFloat((c >> 8) & 255) / 255, alpha: CGFloat(c & 255) / 255)
}

private final class Pin: MKPointAnnotation {
    var id = ""
}

private final class Line: MKPolyline {
    var stroke = UIColor.systemBlue
    var width: CGFloat = 4
}

private final class AppleMapHost: NSObject, NimbusNativeView, MKMapViewDelegate, UIGestureRecognizerDelegate {
    private let map = MKMapView()
    private let emit: (String, String) -> Void
    private var lastCamera = ""
    private var lastShapes = ""
    private var zoom: Float = 12
    private var placed = false

    var view: UIView { map }

    init(_ emit: @escaping (String, String) -> Void) {
        self.emit = emit
        super.init()
        map.delegate = self
        let tap = UITapGestureRecognizer(target: self, action: #selector(tapped(_:)))
        tap.delegate = self
        map.addGestureRecognizer(tap)
    }

    private func place(_ o: Any?) -> CLLocationCoordinate2D? {
        guard let d = o as? [String: Any], let lat = (d["lat"] as? NSNumber)?.doubleValue, let lng = (d["lng"] as? NSNumber)?.doubleValue else { return nil }
        return CLLocationCoordinate2D(latitude: lat, longitude: lng)
    }

    /// Google's zoom: at 0 the world's 360° fills 256 points, and each step halves it.
    private var width: Double { Double(map.bounds.width > 0 ? map.bounds.width : UIScreen.main.bounds.width) }

    private func region(_ at: CLLocationCoordinate2D, _ z: Float) -> MKCoordinateRegion {
        let lng = min(360, 360 / pow(2, Double(z)) * width / 256)
        let tall = Double(map.bounds.height > 0 ? map.bounds.height : map.bounds.width) / max(1, Double(map.bounds.width))
        let lat = min(170, lng * tall * cos(at.latitude * .pi / 180)) // Mercator squeezes degrees of longitude away from the equator
        return MKCoordinateRegion(center: at, span: MKCoordinateSpan(latitudeDelta: lat, longitudeDelta: lng))
    }

    func update(_ p: [String: Any]) {
        let camera = "\(p["center"] ?? "")\(p["zoom"] ?? "")"
        if camera != lastCamera, let at = place(p["center"]) {
            zoom = (p["zoom"] as? NSNumber)?.floatValue ?? 12
            map.setRegion(map.regionThatFits(region(at, zoom)), animated: placed)
            lastCamera = camera
            placed = true
        }
        switch p["kind"] as? String {
        case "satellite": map.mapType = .satellite
        case "hybrid": map.mapType = .hybrid
        default: map.mapType = .standard
        }
        map.showsUserLocation = p["myLocation"] as? Bool ?? false
        let shapes = "\(p["markers"] ?? "")\(p["lines"] ?? "")"
        guard shapes != lastShapes else { return }
        lastShapes = shapes
        map.removeAnnotations(map.annotations.filter { $0 is Pin })
        map.removeOverlays(map.overlays)
        for m in p["markers"] as? [[String: Any]] ?? [] {
            guard let at = place(m["at"]) else { continue }
            let pin = Pin()
            pin.coordinate = at
            pin.title = m["title"] as? String
            pin.subtitle = m["subtitle"] as? String
            pin.id = m["id"] as? String ?? ""
            map.addAnnotation(pin)
        }
        for l in p["lines"] as? [[String: Any]] ?? [] {
            var points = (l["points"] as? [Any] ?? []).compactMap { place($0) }
            guard points.count > 1 else { continue }
            let line = Line(coordinates: &points, count: points.count)
            line.stroke = color((l["color"] as? NSNumber)?.intValue ?? 0)
            line.width = CGFloat((l["width"] as? NSNumber)?.floatValue ?? 4)
            map.addOverlay(line)
        }
    }

    private func json(_ d: [String: Any]) -> String {
        (try? JSONSerialization.data(withJSONObject: d)).map { String(decoding: $0, as: UTF8.self) } ?? "{}"
    }

    /// A tap on a pin is the pin's, not the map's.
    func gestureRecognizer(_ g: UIGestureRecognizer, shouldReceive t: UITouch) -> Bool {
        var v = t.view
        while let x = v, x !== map {
            if x is MKAnnotationView { return false }
            v = x.superview
        }
        return true
    }

    func gestureRecognizer(_ g: UIGestureRecognizer, shouldRecognizeSimultaneouslyWith o: UIGestureRecognizer) -> Bool { true }

    @objc private func tapped(_ g: UITapGestureRecognizer) {
        let c = map.convert(g.location(in: map), toCoordinateFrom: map)
        emit("tap", json(["at": ["lat": c.latitude, "lng": c.longitude]]))
    }

    func mapView(_ m: MKMapView, didSelect v: MKAnnotationView) {
        guard let pin = v.annotation as? Pin else { return }
        emit("marker", json(["id": pin.id]))
    }

    func mapView(_ m: MKMapView, rendererFor o: MKOverlay) -> MKOverlayRenderer {
        guard let line = o as? Line else { return MKOverlayRenderer(overlay: o) }
        let r = MKPolylineRenderer(polyline: line)
        r.strokeColor = line.stroke
        r.lineWidth = line.width
        return r
    }

    func mapView(_ m: MKMapView, regionDidChangeAnimated animated: Bool) {
        let span = max(m.region.span.longitudeDelta, 1e-9)
        let z = log2(360 * width / 256 / span)
        let c = m.region.center
        emit("move", json(["at": ["lat": c.latitude, "lng": c.longitude], "zoom": z]))
    }
}
