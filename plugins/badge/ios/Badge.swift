import UIKit
import UserNotifications

/// badge: the number on the icon. Setting it takes notification permission.
enum BadgePlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "set" else { return Plugins.unknownMethod(call) }
        let n = call.args["count"] as? Int ?? 0
        UNUserNotificationCenter.current().requestAuthorization(options: [.badge]) { granted, _ in
            guard granted else { return call.reject(Plugins.denied, "the badge is not allowed") }
            DispatchQueue.main.async {
                if #available(iOS 16, *) {
                    UNUserNotificationCenter.current().setBadgeCount(n)
                } else {
                    UIApplication.shared.applicationIconBadgeNumber = n
                }
                call.resolve()
            }
        }
    }
}
