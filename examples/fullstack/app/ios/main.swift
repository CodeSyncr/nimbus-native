import UIKit

/// The orders app's iOS host, plus a scripted check against the running
/// server that logs "[fullstack]" lines.
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?
    private let vc = NimbusViewController(schema: NimbusSchema.combined)

    func application(_ app: UIApplication, didFinishLaunchingWithOptions opts: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        window = UIWindow(frame: UIScreen.main.bounds)
        window!.rootViewController = vc
        window!.makeKeyAndVisible()
        Check(vc.renderer).start()
        return true
    }
}

final class Check {
    private let r: Renderer
    init(_ r: Renderer) { self.r = r }

    private var before = 0
    private func count() -> Int { Int(text("count")?.split(separator: " ").first ?? "") ?? 0 }

    func start() {
        waitFor("orders from the server", { self.count() > 0 }) {
            self.before = self.count()
            self.log("list: \(self.text("count")!)")
            self.press("new")
            self.after(1.0) {
                self.press("place") // empty: the server says no
                self.waitFor("server errors", { self.text("error-customer") != nil }) {
                    self.log("empty order: server errors on the form: customer \"\(self.text("error-customer")!)\", phone \"\(self.text("error-phone") ?? "-")\"")
                    self.after(1.5) {
                    self.type("field-customer", "Rohan Das")
                    self.type("field-phone", "90000 11111")
                    self.after(0.5) {
                        self.press("place")
                        self.waitFor("the list to refresh", { self.count() == self.before + 1 }) {
                            self.after(0.8) { // the sheet's dismiss animation
                            let closed = self.r.viewController?.presentedViewController == nil
                            self.log("valid order: sheet closed \(closed), list now \"\(self.text("count")!)\", new row \"\(self.text("order-\(self.before + 1)") ?? "-")\"")
                            self.log(closed && self.text("order-\(self.before + 1)") == "Rohan Das" ? "FULLSTACK RESULT: PASS" : "FULLSTACK RESULT: CHECK")
                            }
                        }
                    }
                    }
                }
            }
        }
    }

    private func text(_ id: String) -> String? { (r.view(forTag: r.tag(forTestID: id) ?? 0) as? UILabel)?.text }
    private func press(_ id: String) { if let t = r.tag(forTestID: id) { r.press(tag: t) } else { log("FAIL: no \(id)") } }
    private func type(_ id: String, _ s: String) {
        guard let f = r.view(forTag: r.tag(forTestID: id) ?? 0) as? UITextField else { return log("FAIL: no \(id)") }
        f.text = s
        f.sendActions(for: .editingChanged)
    }
    private func after(_ s: Double, _ f: @escaping () -> Void) { DispatchQueue.main.asyncAfter(deadline: .now() + s, execute: f) }
    private func waitFor(_ what: String, _ cond: @escaping () -> Bool, tries: Int = 100, _ then: @escaping () -> Void) {
        if cond() { return then() }
        if tries == 0 { return log("FAIL: timed out waiting for \(what)") }
        after(0.1) { self.waitFor(what, cond, tries: tries - 1, then) }
    }
    private func log(_ s: String) { print("[fullstack] " + s) }
}

UIApplicationMain(CommandLine.argc, CommandLine.unsafeArgv, nil, NSStringFromClass(AppDelegate.self))
