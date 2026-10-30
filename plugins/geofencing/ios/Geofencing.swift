import CoreLocation

/// geofencing: Core Location's region monitoring. Crossings wait in
/// UserDefaults, so ones that wake the app in the background aren't lost.
final class GeofencingPlugin: NSObject, CLLocationManagerDelegate {
    private static let shared = GeofencingPlugin()
    private static let key = "nimbus.geofence.events"
    private let manager = CLLocationManager()
    private var asking: [PluginCall] = []

    private override init() {
        super.init()
        manager.delegate = self
    }

    static func boot() { _ = shared }

    static func handle(_ call: PluginCall) { shared.handle(call) }

    private func handle(_ call: PluginCall) {
        switch call.method {
        case "request":
            switch manager.authorizationStatus {
            case .authorizedAlways: call.resolve(["granted": true])
            case .denied, .restricted: call.resolve(["granted": false])
            default:
                asking.append(call)
                manager.requestAlwaysAuthorization()
            }
        case "add":
            guard let id = call.string("id"), let lat = call.args["latitude"] as? Double, let lng = call.args["longitude"] as? Double,
                  let r = call.args["radius"] as? Double else { return call.reject(Plugins.failed, "a region needs an id, a place and a radius") }
            guard CLLocationManager.isMonitoringAvailable(for: CLCircularRegion.self) else {
                return call.reject(Plugins.unavailable, "this phone can't watch regions")
            }
            let region = CLCircularRegion(center: CLLocationCoordinate2D(latitude: lat, longitude: lng), radius: r, identifier: id)
            manager.startMonitoring(for: region)
            call.resolve()
        case "remove":
            manager.monitoredRegions.filter { $0.identifier == call.string("id") }.forEach { manager.stopMonitoring(for: $0) }
            call.resolve()
        case "events":
            let d = UserDefaults.standard
            call.resolve(["events": d.array(forKey: Self.key) ?? []])
            d.removeObject(forKey: Self.key)
        default:
            Plugins.unknownMethod(call)
        }
    }

    private func record(_ region: CLRegion, enter: Bool) {
        let d = UserDefaults.standard
        var list = d.array(forKey: Self.key) ?? []
        list.append(["id": region.identifier, "enter": enter, "time": Date().timeIntervalSince1970])
        d.set(list, forKey: Self.key)
    }

    func locationManager(_ m: CLLocationManager, didEnterRegion r: CLRegion) { record(r, enter: true) }
    func locationManager(_ m: CLLocationManager, didExitRegion r: CLRegion) { record(r, enter: false) }

    func locationManagerDidChangeAuthorization(_ m: CLLocationManager) {
        guard m.authorizationStatus != .notDetermined else { return }
        let ok = m.authorizationStatus == .authorizedAlways || m.authorizationStatus == .authorizedWhenInUse
        asking.forEach { $0.resolve(["granted": ok]) }
        asking = []
    }
}
