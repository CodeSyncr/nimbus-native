import UIKit
import NimbusNativeKit

/// The iOS host. The app itself is Go (app/mobile), built into libapp.a by
/// the "Build Go" phase; this file only gives it a window.
@main
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?
    private var nimbus: NimbusViewController?

    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        let vc = NimbusViewController(schema: NimbusSchema.combined)
        nimbus = vc
        window = UIWindow(frame: UIScreen.main.bounds)
        window!.rootViewController = vc
        window!.makeKeyAndVisible()
        return true
    }

    /// Links to the app (__MODULE__://orders/42) open their page.
    func application(_ app: UIApplication, open url: URL, options: [UIApplication.OpenURLOptionsKey: Any] = [:]) -> Bool {
        nimbus?.renderer.open(url)
        return true
    }
}
