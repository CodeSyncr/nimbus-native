import AppTrackingTransparency

/// trackingconsent: App Tracking Transparency.
enum TrackingConsentPlugin {
    private static func name(_ s: ATTrackingManager.AuthorizationStatus) -> String {
        switch s {
        case .authorized: return "granted"
        case .denied: return "denied"
        case .restricted: return "restricted"
        default: return "not-asked"
        }
    }

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "status":
            call.resolve(["status": name(ATTrackingManager.trackingAuthorizationStatus)])
        case "request":
            ATTrackingManager.requestTrackingAuthorization { s in call.resolve(["status": name(s)]) }
        default:
            Plugins.unknownMethod(call)
        }
    }
}
