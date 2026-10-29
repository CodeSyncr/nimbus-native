import UIKit

/// The gallery's iOS host: one NimbusViewController, plus a benchmark that
/// runs by itself after launch and logs "[gallery]" lines.
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?
    private let vc = NimbusViewController()
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
    private var link: CADisplayLink?
    private var frameTimes: [CFTimeInterval] = []

    init(renderer: Renderer) {
        r = renderer
        r.onCommit = { [weak self] c, applyMs in self?.committed(c, applyMs) }
    }

    private func committed(_ c: Render.Commit, _ applyMs: Double) {
        commits += 1
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
            log(String(format: "scroll: 180 frames through the list, %.1f fps, %d dropped (target %.1f ms/frame)", fps, dropped, target * 1000))
            let median = tapLatencies.sorted()[10]
            log(median < 16 && dropped < 9 ? "NN-0D RESULT: PASS" : "NN-0D RESULT: CHECK (see numbers above)")
        }
    }

    private func log(_ s: String) { print("[gallery] " + s) }
}

UIApplicationMain(CommandLine.argc, CommandLine.unsafeArgv, nil, NSStringFromClass(AppDelegate.self))
