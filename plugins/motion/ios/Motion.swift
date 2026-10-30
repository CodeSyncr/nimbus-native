import CoreMotion

/// motion: Core Motion, read as Go asks.
enum MotionPlugin {
    private static let manager = CMMotionManager()

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "start":
            guard manager.isAccelerometerAvailable else { return call.reject(Plugins.unavailable, "this phone has no motion sensors") }
            let every = 1 / Double(call.args["hz"] as? Int ?? 30)
            manager.accelerometerUpdateInterval = every
            manager.gyroUpdateInterval = every
            manager.startAccelerometerUpdates()
            if manager.isGyroAvailable { manager.startGyroUpdates() }
            call.resolve()
        case "read":
            let a = manager.accelerometerData?.acceleration ?? CMAcceleration()
            let g = manager.gyroData?.rotationRate ?? CMRotationRate()
            call.resolve(["accel": ["x": a.x, "y": a.y, "z": a.z], "gyro": ["x": g.x, "y": g.y, "z": g.z]])
        case "stop":
            manager.stopAccelerometerUpdates()
            manager.stopGyroUpdates()
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
