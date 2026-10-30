import CoreLocation

/// geolocation: Core Location, while the app is open.
final class GeolocationPlugin: NSObject, CLLocationManagerDelegate {
    private static let shared = GeolocationPlugin()
    private let manager = CLLocationManager()
    private var asking: [PluginCall] = []
    private var waiting: [PluginCall] = []
    private var latest: CLLocation?

    private override init() {
        super.init()
        manager.delegate = self
        manager.desiredAccuracy = kCLLocationAccuracyBest
    }

    static func handle(_ call: PluginCall) { shared.handle(call) }

    private var allowed: Bool {
        let s = manager.authorizationStatus
        return s == .authorizedWhenInUse || s == .authorizedAlways
    }

    private func handle(_ call: PluginCall) {
        switch call.method {
        case "request":
            if manager.authorizationStatus != .notDetermined { return call.resolve(["granted": allowed]) }
            asking.append(call)
            manager.requestWhenInUseAuthorization()
        case "current":
            guard allowed else { return call.reject(Plugins.denied, "the location is not allowed") }
            waiting.append(call)
            manager.requestLocation()
        case "start":
            guard allowed else { return call.reject(Plugins.denied, "the location is not allowed") }
            manager.startUpdatingLocation()
            call.resolve()
        case "stop":
            manager.stopUpdatingLocation()
            call.resolve()
        case "latest":
            guard let l = latest else { return call.resolve(["known": false]) }
            var r = Self.position(l)
            r["known"] = true
            call.resolve(r)
        default:
            Plugins.unknownMethod(call)
        }
    }

    private static func position(_ l: CLLocation) -> [String: Any] {
        ["latitude": l.coordinate.latitude, "longitude": l.coordinate.longitude, "accuracy": l.horizontalAccuracy,
         "altitude": l.altitude, "speed": l.speed, "heading": l.course]
    }

    func locationManagerDidChangeAuthorization(_ m: CLLocationManager) {
        guard m.authorizationStatus != .notDetermined else { return }
        asking.forEach { $0.resolve(["granted": allowed]) }
        asking = []
    }

    func locationManager(_ m: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        guard let l = locations.last else { return }
        latest = l
        waiting.forEach { $0.resolve(Self.position(l)) }
        waiting = []
    }

    func locationManager(_ m: CLLocationManager, didFailWithError error: Error) {
        waiting.forEach { $0.reject(Plugins.failed, error.localizedDescription) }
        waiting = []
    }
}
