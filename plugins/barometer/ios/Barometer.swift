import CoreMotion

/// barometer: the altimeter, read as Go asks. Core Motion gives kilopascals.
enum BarometerPlugin {
    private static let altimeter = CMAltimeter()
    private static var pressure = 0.0
    private static var altitude = 0.0

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "available":
            call.resolve(["available": CMAltimeter.isRelativeAltitudeAvailable()])
        case "start":
            guard CMAltimeter.isRelativeAltitudeAvailable() else { return call.reject(Plugins.unavailable, "this phone has no barometer") }
            altimeter.startRelativeAltitudeUpdates(to: .main) { d, _ in
                guard let d else { return }
                pressure = d.pressure.doubleValue * 10
                altitude = d.relativeAltitude.doubleValue
            }
            call.resolve()
        case "read":
            call.resolve(["pressure": pressure, "altitude": altitude])
        case "stop":
            altimeter.stopRelativeAltitudeUpdates()
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
