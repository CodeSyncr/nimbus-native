import UIKit

/// The gallery's iOS host: one NimbusViewController, plus a benchmark that
/// runs by itself after launch and logs "[gallery]" lines.
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?
    private let vc = NimbusViewController(schema: NimbusSchema.combined)
    private var bench: Bench?

    func application(_ app: UIApplication, didFinishLaunchingWithOptions opts: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        window = UIWindow(frame: UIScreen.main.bounds)
        window!.rootViewController = vc
        window!.makeKeyAndVisible()
        bench = Bench(renderer: vc.renderer)
        return true
    }
}

/// Measures the first render, tap → screen latency, and scrolling.
final class Bench {
    private let r: Renderer
    private var commits = 0
    private var pressAt: CFTimeInterval = 0
    private var tapLatencies: [Double] = []
    private var typing = false
    private var scrolling = false, scrollCommits = 0
    private var listOK = false
    private var pushing = false
    private var link: CADisplayLink?
    private var frameTimes: [CFTimeInterval] = []

    init(renderer: Renderer) {
        r = renderer
        r.onCommit = { [weak self] c, applyMs in self?.committed(c, applyMs) }
    }

    private func committed(_ c: Render.Commit, _ applyMs: Double) {
        commits += 1
        if scrolling { scrollCommits += 1 }
        if pushing {
            pushing = false
            log(String(format: "push: row tap → detail screen committed in %.2f ms (%d ops; Go build %.2f ms, layout %.2f ms with %d text measurements; apply incl. push %.2f ms)",
                       (CACurrentMediaTime() - pressAt) * 1000, c.ops.count, Double(c.buildNs) / 1e6, Double(c.layoutNs) / 1e6, c.measures, applyMs))
            pressAt = 0
            return
        }
        if typing {
            typing = false
            let greeting = (r.view(forTag: r.tag(forTestID: "greeting") ?? 0) as? UILabel)?.text ?? "?"
            log("typed \"Yash\" → greeting: \(greeting)")
            if greeting != "Hello, Yash 👋" { return log("FAIL: greeting did not follow the input") }
            return scroll()
        }
        if commits == 1 {
            let views = c.ops.filter { if case .create? = $0 { return true }; return false }.count
            log(String(format: "first render: %d native views, %d ops; Go build %.1f ms, layout %.1f ms (%d text measurements); UIKit apply %.1f ms",
                       views, c.ops.count, Double(c.buildNs) / 1e6, Double(c.layoutNs) / 1e6, c.measures, applyMs))
            DispatchQueue.main.asyncAfter(deadline: .now() + 1) { self.tap() }
            return
        }
        if pressAt > 0 {
            tapLatencies.append((CACurrentMediaTime() - pressAt) * 1000)
            if tapLatencies.count == 1 {
                log(String(format: "tap commit: %d ops; Go build %.2f ms, layout %.2f ms; apply %.2f ms",
                           c.ops.count, Double(c.buildNs) / 1e6, Double(c.layoutNs) / 1e6, applyMs))
            }
            pressAt = 0
            if tapLatencies.count < 20 {
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.05) { self.tap() }
            } else {
                let s = tapLatencies.sorted()
                log(String(format: "tap → screen: 20 taps, median %.2f ms, worst %.2f ms", s[10], s[19]))
                let label = (r.view(forTag: r.tag(forTestID: "counter") ?? 0)?.subviews.first as? UILabel)?.text ?? "?"
                log("counter label now: \(label)")
                typeName()
            }
        }
    }

    private func tap() {
        guard let tag = r.tag(forTestID: "counter") else { return log("FAIL: no counter") }
        pressAt = CACurrentMediaTime()
        r.press(tag: tag)
    }

    /// Types into the name field the way the keyboard does.
    private func typeName() {
        guard let f = r.view(forTag: r.tag(forTestID: "name") ?? 0) as? UITextField else { return log("FAIL: no name field") }
        typing = true
        f.text = "Yash"
        f.sendActions(for: .editingChanged)
    }

    private func scroll() {
        guard let tag = r.tag(forTestID: "list"), let s = r.view(forTag: tag) as? UIScrollView else { return log("FAIL: no list") }
        log(String(format: "list content: %.0f pt tall", s.contentSize.height))
        scrolling = true
        link = CADisplayLink(target: self, selector: #selector(step))
        link!.add(to: .main, forMode: .common)
    }

    @objc private func step(_ l: CADisplayLink) {
        frameTimes.append(l.timestamp)
        guard let s = r.view(forTag: r.tag(forTestID: "list") ?? 0) as? UIScrollView else { return }
        s.contentOffset.y = min(s.contentOffset.y + 40, s.contentSize.height - s.bounds.height)
        if frameTimes.count == 180 {
            l.invalidate()
            let intervals = zip(frameTimes.dropFirst(), frameTimes).map { $0 - $1 }
            let target = l.targetTimestamp - l.timestamp
            let dropped = intervals.filter { $0 > target * 1.5 }.count
            let fps = Double(intervals.count) / (frameTimes.last! - frameTimes.first!)
            log(String(format: "scroll: 180 frames through the list, %.1f fps, %d dropped (target %.1f ms/frame), %d window commits",
                       fps, dropped, target * 1000, scrollCommits))
            scrolling = false
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { [self] in
                // Rows (views with content; spacers have none) must cover the visible area.
                let visible = CGRect(origin: s.contentOffset, size: s.bounds.size)
                let rows = s.subviews.filter { !$0.subviews.isEmpty && $0.frame.intersects(visible) }
                let covered = rows.map { $0.frame.maxY }.max() ?? 0
                log(String(format: "after scroll: %d rows on screen, rows reach %.0f of %.0f pt; %d native views in the list",
                           rows.count, covered - visible.minY, visible.height, s.subviews.count))
                listOK = tapLatencies.sorted()[10] < 16 && dropped < 9 && covered >= visible.maxY
                navigate(s)
            }
        }
    }

    /// Taps the first row on screen, then goes back the way a user does.
    private func navigate(_ list: UIScrollView) {
        let visible = CGRect(origin: list.contentOffset, size: list.bounds.size)
        guard let row = list.subviews.first(where: { ($0.accessibilityIdentifier ?? "").hasPrefix("row-") && $0.frame.intersects(visible) }),
              let stack = r.view(forTag: r.tag(forTestID: "stack") ?? 0) as? NNStackView else { return log("FAIL: no row or stack") }
        pushing = true
        pressAt = CACurrentMediaTime()
        r.press(tag: UInt32(row.tag))
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [self] in
            let title = r.view(forTag: r.tag(forTestID: "detail-title") ?? 0) as? UILabel
            log("after push: \(stack.nav.viewControllers.count) screens, bar title \"\(stack.nav.topViewController?.title ?? "")\", detail shows \(title?.text ?? "nothing")")
            let pushed = stack.nav.viewControllers.count == 2 && title != nil
            stack.nav.popViewController(animated: true) // what the back button and swipe do
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [self] in
                let goFollowed = r.tag(forTestID: "detail-title") == nil
                log("after native back: \(stack.nav.viewControllers.count) screen, Go dropped the detail screen: \(goFollowed)")
                switchTabs(listOK && pushed && goFollowed)
            }
        }
    }

    /// Taps the Account tab the way a user does, then switches back from Go.
    private func switchTabs(_ okSoFar: Bool) {
        guard let tabs = r.view(forTag: r.tag(forTestID: "tabs") ?? 0) as? NNTabsView else { return log("FAIL: no tabs") }
        tabs.bar.selectedIndex = 1
        tabs.bar.delegate?.tabBarController?(tabs.bar, didSelect: tabs.bar.viewControllers![1])
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) { [self] in
            let account = (r.view(forTag: r.tag(forTestID: "account-title") ?? 0) as? UILabel)?.text ?? "nothing"
            log("user picked Account: tab \(tabs.bar.selectedIndex) shows \(account)")
            guard let button = r.tag(forTestID: "to-orders") else { return log("FAIL: no button") }
            r.press(tag: button)
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) { [self] in
                log("Go selected Orders: tab bar now on tab \(tabs.bar.selectedIndex)")
                let ok = okSoFar && account == "Asha Rao" && tabs.bar.selectedIndex == 0
                modalTest(ok)
            }
        }
    }

    /// Opens the New order sheet, swipes it away as a user does, then opens it
    /// again and closes it from Go.
    private func modalTest(_ okSoFar: Bool) {
        guard let open = r.tag(forTestID: "new-order"), let host = r.viewController else { return log("FAIL: no button") }
        r.press(tag: open)
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [self] in
            let form = r.tag(forTestID: "new-order-form") != nil
            guard let sheet = host.presentedViewController else { return log("FAIL: nothing presented") }
            log("modal: sheet presented, form showing: \(form)")
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) { [self] in
            // What a swipe down does: UIKit dismisses, then tells the delegate.
            let pc = sheet.presentationController
            sheet.dismiss(animated: true) { if let pc { pc.delegate?.presentationControllerDidDismiss?(pc) } }
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [self] in
                let swiped = r.tag(forTestID: "new-order-form") == nil
                log("modal: swiped away, Go dropped it: \(swiped)")
                r.press(tag: open)
                DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [self] in
                    guard let place = r.tag(forTestID: "place-order") else { return log("FAIL: reopen") }
                    r.press(tag: place) // empty form: errors, sheet stays
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.6) { [self] in
                        let errors = ["customer", "phone"].filter { r.tag(forTestID: "error-" + $0) != nil }
                        let stayed = host.presentedViewController != nil
                        log("form: empty submit shows errors for \(errors), sheet still open: \(stayed)")
                        type("field-customer", "Asha Rao")
                        type("field-phone", "98765 43210")
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.6) { [self] in
                            let cleared = r.tag(forTestID: "error-customer") == nil && r.tag(forTestID: "error-phone") == nil
                            r.press(tag: place)
                            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [self] in
                                let closed = host.presentedViewController == nil
                                log("form: valid submit cleared errors: \(cleared), placed and dismissed: \(closed)")
                                let ok = okSoFar && form && swiped && errors.count == 2 && stayed && cleared && closed
                                log(ok ? "NN-P1 RESULT: PASS" : "NN-P1 RESULT: CHECK (see numbers above)")
                            }
                        }
                    }
                }
            }
            }
        }
    }

    /// Types into a text field the way the keyboard does.
    private func type(_ testID: String, _ text: String) {
        guard let f = r.view(forTag: r.tag(forTestID: testID) ?? 0) as? UITextField else { return log("FAIL: no \(testID)") }
        f.text = text
        f.sendActions(for: .editingChanged)
    }

    private func log(_ s: String) { print("[gallery] " + s) }
}

UIApplicationMain(CommandLine.argc, CommandLine.unsafeArgv, nil, NSStringFromClass(AppDelegate.self))
