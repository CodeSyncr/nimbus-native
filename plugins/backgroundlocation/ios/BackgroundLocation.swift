import CoreLocation

/// backgroundlocation: a manager of its own, allowed to run in the background, with the indicator on.
final class BackgroundLocationPlugin: NSObject, CLLocationManagerDelegate {
    private static let shared = BackgroundLocationPlugin()
    private static let limit = 5000
    private let manager = CLLocationManager()
    private var asking: [PluginCall] = []
    private var points: [[String: Any]] = []
    private var running = false

    private override init() {
        super.init()
        manager.delegate = self
        manager.desiredAccuracy = kCLLocationAccuracyBest
        manager.pausesLocationUpdatesAutomatically = false
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
        case "start":
            guard allowed else { return call.reject(Plugins.denied, "the location is not allowed") }
            manager.distanceFilter = call.args["minDistance"] as? Double ?? 10
            manager.allowsBackgroundLocationUpdates = true
            manager.showsBackgroundLocationIndicator = true
            manager.startUpdatingLocation()
            running = true
            call.resolve()
        case "stop":
            manager.stopUpdatingLocation()
            manager.allowsBackgroundLocationUpdates = false
            running = false
            call.resolve()
        case "running":
            call.resolve(["running": running])
        case "points":
            call.resolve(points)
            points = []
        default:
            Plugins.unknownMethod(call)
        }
    }

    func locationManagerDidChangeAuthorization(_ m: CLLocationManager) {
        guard m.authorizationStatus != .notDetermined else { return }
        asking.forEach { $0.resolve(["granted": allowed]) }
        asking = []
    }

    func locationManager(_ m: CLLocationManager, didUpdateLocations list: [CLLocation]) {
        for l in list {
            points.append(["latitude": l.coordinate.latitude, "longitude": l.coordinate.longitude, "accuracy": l.horizontalAccuracy,
                           "speed": l.speed, "time": l.timestamp.timeIntervalSince1970])
        }
        if points.count > Self.limit { points.removeFirst(points.count - Self.limit) }
    }

    func locationManager(_ m: CLLocationManager, didFailWithError e: Error) {} // it keeps trying
}
