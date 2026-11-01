import CoreMotion

/// pedometer: CMPedometer. Asking for a count is what shows the permission prompt.
enum PedometerPlugin {
    private static let pedometer = CMPedometer()
    private static var steps = 0

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "available":
            call.resolve(["available": CMPedometer.isStepCountingAvailable()])
        case "request":
            guard CMPedometer.isStepCountingAvailable() else { return call.reject(Plugins.unavailable, "this phone can't count steps") }
            pedometer.queryPedometerData(from: Date().addingTimeInterval(-60), to: Date()) { _, _ in
                call.resolve(["granted": CMPedometer.authorizationStatus() == .authorized])
            }
        case "start":
            guard CMPedometer.isStepCountingAvailable() else { return call.reject(Plugins.unavailable, "this phone can't count steps") }
            steps = 0
            pedometer.startUpdates(from: Date()) { d, _ in
                if let d { steps = d.numberOfSteps.intValue }
            }
            call.resolve()
        case "read":
            call.resolve(["steps": steps])
        case "stop":
            pedometer.stopUpdates()
            call.resolve()
        case "between":
            guard CMPedometer.isStepCountingAvailable() else { return call.reject(Plugins.unavailable, "this phone can't count steps") }
            let from = Date(timeIntervalSince1970: call.args["from"] as? Double ?? 0)
            let to = Date(timeIntervalSince1970: call.args["to"] as? Double ?? 0)
            pedometer.queryPedometerData(from: from, to: to) { d, e in
                if let d { call.resolve(["steps": d.numberOfSteps.intValue]) }
                else { call.reject(Plugins.failed, e?.localizedDescription ?? "no steps for then") }
            }
        default:
            Plugins.unknownMethod(call)
        }
    }
}
