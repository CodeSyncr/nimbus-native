import ObjectiveC
import UIKit
import UserNotifications

/// push: APNs. The registration answers arrive at the app delegate, so boot
/// adds those two methods to it, and the project's AppDelegate stays as it is.
final class PushPlugin: NSObject, UNUserNotificationCenterDelegate {
    private static let shared = PushPlugin()
    private static var token: String?
    private static var failure: String?
    private static var waiting: [PluginCall] = []
    private static var inbox: [[String: Any]] = []

    static func boot() {
        UNUserNotificationCenter.current().delegate = shared
        DispatchQueue.main.async {
            guard let cls = (UIApplication.shared.delegate as? NSObject).map({ type(of: $0) }) else { return }
            let got: @convention(block) (AnyObject, UIApplication, Data) -> Void = { _, _, d in
                token = d.map { String(format: "%02x", $0) }.joined()
                answer()
            }
            let failed: @convention(block) (AnyObject, UIApplication, NSError) -> Void = { _, _, e in
                failure = e.localizedDescription
                answer()
            }
            class_addMethod(cls, NSSelectorFromString("application:didRegisterForRemoteNotificationsWithDeviceToken:"),
                            imp_implementationWithBlock(got), "v@:@@")
            class_addMethod(cls, NSSelectorFromString("application:didFailToRegisterForRemoteNotificationsWithError:"),
                            imp_implementationWithBlock(failed), "v@:@@")
        }
    }

    private static func answer() {
        let calls = waiting
        waiting = []
        for c in calls {
            if let token { c.resolve(["value": token, "service": "apns"]) } else { c.reject(Plugins.failed, failure ?? "no token") }
        }
    }

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "request":
            UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { ok, _ in call.resolve(["granted": ok]) }
        case "token":
            if let token { return call.resolve(["value": token, "service": "apns"]) }
            waiting.append(call)
            UIApplication.shared.registerForRemoteNotifications()
        case "messages":
            call.resolve(["messages": inbox])
            inbox = []
        default:
            Plugins.unknownMethod(call)
        }
    }

    private static func keep(_ n: UNNotification, tapped: Bool) {
        guard n.request.trigger is UNPushNotificationTrigger else { return } // local ones aren't push
        var data: [String: String] = [:]
        for (k, v) in n.request.content.userInfo where (k as? String) != "aps" {
            if let k = k as? String { data[k] = "\(v)" }
        }
        let c = n.request.content
        inbox.append(["title": c.title, "body": c.body, "data": data, "tapped": tapped])
    }

    func userNotificationCenter(_ c: UNUserNotificationCenter, willPresent n: UNNotification,
                                withCompletionHandler done: @escaping (UNNotificationPresentationOptions) -> Void) {
        DispatchQueue.main.async { Self.keep(n, tapped: false) }
        done([.banner, .sound])
    }

    func userNotificationCenter(_ c: UNUserNotificationCenter, didReceive r: UNNotificationResponse,
                                withCompletionHandler done: @escaping () -> Void) {
        DispatchQueue.main.async { Self.keep(r.notification, tapped: true) }
        done()
    }
}
