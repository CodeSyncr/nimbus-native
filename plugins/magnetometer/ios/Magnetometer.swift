import CoreMotion

/// magnetometer: Core Motion's raw field, read as Go asks.
enum MagnetometerPlugin {
    private static let manager = CMMotionManager()

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "available":
            call.resolve(["available": manager.isMagnetometerAvailable])
        case "start":
            guard manager.isMagnetometerAvailable else { return call.reject(Plugins.unavailable, "this phone has no magnetometer") }
            manager.magnetometerUpdateInterval = 1 / Double(call.args["hz"] as? Int ?? 10)
            manager.startMagnetometerUpdates()
            call.resolve()
        case "read":
            let m = manager.magnetometerData?.magneticField ?? CMMagneticField()
            call.resolve(["x": m.x, "y": m.y, "z": m.z])
        case "stop":
            manager.stopMagnetometerUpdates()
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
