import UIKit
import NimbusNativeKit

/// The iOS host. The app itself is Go (app/mobile), built into libapp.a by
/// the "Build Go" phase; this file only gives it a window.
@main
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?

    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        window = UIWindow(frame: UIScreen.main.bounds)
        window!.rootViewController = NimbusViewController(schema: NimbusSchema.combined)
        window!.makeKeyAndVisible()
        return true
    }
}
