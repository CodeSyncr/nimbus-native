import UserNotifications

/// localnotifications: shown even with the app open.
final class LocalNotificationsPlugin: NSObject, UNUserNotificationCenterDelegate {
    private static let shared = LocalNotificationsPlugin()

    static func handle(_ call: PluginCall) {
        let center = UNUserNotificationCenter.current()
        if center.delegate == nil { center.delegate = shared } // push may have it
        switch call.method {
        case "request":
            center.requestAuthorization(options: [.alert, .sound, .badge]) { ok, _ in call.resolve(["granted": ok]) }
        case "schedule":
            let c = UNMutableNotificationContent()
            c.title = call.string("title") ?? ""
            c.body = call.string("body") ?? ""
            c.sound = .default
            let t = UNTimeIntervalNotificationTrigger(timeInterval: call.args["seconds"] as? Double ?? 1, repeats: false)
            center.add(UNNotificationRequest(identifier: call.string("id") ?? UUID().uuidString, content: c, trigger: t)) { e in
                if let e { call.reject(Plugins.failed, e.localizedDescription) } else { call.resolve() }
            }
        case "cancel":
            center.removePendingNotificationRequests(withIdentifiers: [call.string("id") ?? ""])
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }

    func userNotificationCenter(_ c: UNUserNotificationCenter, willPresent n: UNNotification,
                                withCompletionHandler done: @escaping (UNNotificationPresentationOptions) -> Void) {
        done([.banner, .sound])
    }
}
