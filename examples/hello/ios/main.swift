import UIKit

/// Phase 0a checks, run at launch. Every line also goes to the console so
/// `nimbus-native run ios` can read the result.
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?
    private let label = UILabel()
    private var lines: [String] = []
    private let bridge = Bridge.shared

    private var pings = 0, pingStart = DispatchTime.now()
    private var ticks = 0, readyAt: DispatchTime?
    private let pingCount = 1000

    func application(_ app: UIApplication, didFinishLaunchingWithOptions opts: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        let vc = UIViewController()
        vc.view.backgroundColor = .systemBackground
        label.numberOfLines = 0
        label.font = .monospacedSystemFont(ofSize: 13, weight: .regular)
        label.translatesAutoresizingMaskIntoConstraints = false
        vc.view.addSubview(label)
        NSLayoutConstraint.activate([
            label.leadingAnchor.constraint(equalTo: vc.view.leadingAnchor, constant: 16),
            label.trailingAnchor.constraint(equalTo: vc.view.trailingAnchor, constant: -16),
            label.topAnchor.constraint(equalTo: vc.view.safeAreaLayoutGuide.topAnchor, constant: 16),
        ])
        window = UIWindow(frame: UIScreen.main.bounds)
        window!.rootViewController = vc
        window!.makeKeyAndVisible()

        run()
        return true
    }

    private func run() {
        let t0 = DispatchTime.now()
        bridge.onReceive = { [weak self] in self?.received(String(decoding: $0, as: UTF8.self)) }
        do {
            try bridge.start()
            log("start: ok in \(ms(since: t0)) ms")

            log("call: " + String(decoding: try bridge.call(Data("Yash".utf8)), as: UTF8.self))

            let n = 10_000, b0 = DispatchTime.now()
            for _ in 0..<n { _ = try bridge.call(Data("bench".utf8)) }
            log(String(format: "sync calls: %d, avg %.2f µs", n, us(since: b0) / Double(n)))

            do {
                _ = try bridge.call(Data("panic".utf8))
                log("panic: FAIL (no error)")
            } catch let e as BridgeError where e.code == NN_ERR_PANIC {
                log("panic: recovered (\(e.detail.split(separator: "\n").first ?? ""))")
            }

            pingStart = DispatchTime.now()
            try bridge.send(Data("ping:0".utf8))
        } catch {
            log("FAIL: \(error)")
        }
    }

    private func received(_ msg: String) {
        if msg == "ready" {
            log("go → native: ready")
        } else if msg.hasPrefix("tick:") {
            ticks += 1
            log("go → native: \(msg) (goroutine)")
        } else if msg.hasPrefix("pong:") {
            pings += 1
            if pings < pingCount {
                try? bridge.send(Data("ping:\(pings)".utf8))
            } else {
                log(String(format: "async ping-pong: %d, avg %.2f µs round trip", pingCount, us(since: pingStart) / Double(pingCount)))
            }
        }
        if ticks == 3 && pings == pingCount {
            ticks += 1 // report once
            log("NN-0A RESULT: PASS")
        }
    }

    private func log(_ s: String) {
        print("[hello] " + s)
        lines.append(s)
        label.text = "Nimbus Native · Phase 0a\n\n" + lines.joined(separator: "\n")
    }

    private func us(since t: DispatchTime) -> Double { Double(DispatchTime.now().uptimeNanoseconds - t.uptimeNanoseconds) / 1_000 }
    private func ms(since t: DispatchTime) -> String { String(format: "%.1f", us(since: t) / 1_000) }
}

UIApplicationMain(CommandLine.argc, CommandLine.unsafeArgv, nil, NSStringFromClass(AppDelegate.self))
