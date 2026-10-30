import LocalAuthentication

/// biometrics: Local Authentication.
enum BiometricsPlugin {
    static func handle(_ call: PluginCall) {
        let c = LAContext()
        var e: NSError?
        let can = c.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &e)
        switch call.method {
        case "available":
            guard can else { return call.resolve(["kind": ""]) }
            call.resolve(["kind": c.biometryType == .faceID ? "face" : c.biometryType == .touchID ? "fingerprint" : ""])
        case "confirm":
            guard can else { return call.reject(Plugins.unavailable, "no biometrics are set up on this phone") }
            c.evaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, localizedReason: call.string("reason") ?? "Confirm it's you") { ok, _ in
                call.resolve(["ok": ok])
            }
        default:
            Plugins.unknownMethod(call)
        }
    }
}
