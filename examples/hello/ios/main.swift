import UIKit

/// Bridge checks, run at launch. Every line also goes to the console so
/// `nimbus-native run ios` can read the result.
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?
    private let label = UILabel()
    private var lines: [String] = []
    private let bridge = Bridge.shared

    private enum Stage { case ping, burst, soak(round: Int), done }
    private var stage = Stage.ping
    private var t0 = DispatchTime.now()
    private var ticks = 0, pings = 0, burstNext = 0, burstTarget = 0, drainsBefore = 0
    private var soakOK = false
    private var soakMem: [(heap: Int, footprint: Int, goroutines: Int)] = []

    private let pingCount = 1000, burstCount = 100_000, soakCalls = 100_000

    func application(_ app: UIApplication, didFinishLaunchingWithOptions opts: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        let vc = UIViewController()
        vc.view.backgroundColor = .systemBackground
        label.numberOfLines = 0
        label.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
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

        bridge.onReceive = { [weak self] in self?.received(String(decoding: $0, as: UTF8.self)) }
        do {
            try run()
        } catch {
            log("FAIL: \(error)")
        }
        return true
    }

    private func run() throws {
        t0 = .now()
        try bridge.start()
        log("start: ok in \(fmt(ms(since: t0))) ms")
        log("call: " + (try text("hello:Yash")))

        let n = 10_000, b0 = DispatchTime.now()
        for _ in 0..<n { _ = try bridge.call(Data("echo:x".utf8)) }
        log("sync calls: \(n), avg \(fmt(us(since: b0) / Double(n))) µs")

        do {
            _ = try bridge.call(Data("panic".utf8))
            log("FAIL: panic not reported")
        } catch let e as BridgeError where e.code == NN_ERR_PANIC {
            let carriesStack = e.detail.contains("asked to panic") && e.detail.contains("goroutine")
            log("panic: recovered, message + Go stack \(carriesStack ? "delivered" : "MISSING")")
        }

        t0 = .now()
        try bridge.send(Data("ping:0".utf8))
    }

    private func received(_ msg: String) {
        if msg == "ready" { return log("go → native: ready") }
        if msg.hasPrefix("tick:") { ticks += 1; log("go → native: \(msg) (goroutine)"); return maybePass() }

        switch stage {
        case .ping where msg.hasPrefix("pong:"):
            pings += 1
            if pings < pingCount {
                try? bridge.send(Data("ping:\(pings)".utf8))
            } else {
                log("ping-pong: \(pingCount) round trips, avg \(fmt(us(since: t0) / Double(pingCount))) µs")
                startBurst(next: .burst)
            }
        case .burst, .soak:
            guard msg == "b:\(burstNext)" else { return fail("burst out of order: got \(msg), want b:\(burstNext)") }
            burstNext += 1
            if burstNext == burstTarget { burstDone() }
        default:
            break
        }
    }

    private func startBurst(next: Stage) {
        stage = next
        burstNext = 0
        burstTarget = burstCount
        drainsBefore = stats(gc: false)[4]
        t0 = .now()
        _ = try? bridge.call(Data("burst:\(burstCount)".utf8))
    }

    private func burstDone() {
        let drains = stats(gc: false)[4] - drainsBefore
        switch stage {
        case .burst:
            log("burst: \(burstCount) msgs in order, \(drains) drains (\(burstCount / max(drains, 1)) msgs/drain), \(fmt(us(since: t0) * 1000 / Double(burstCount))) ns/msg")
            soakRound(1)
        case .soak(let round):
            let s = stats(gc: true)
            soakMem.append((s[0], footprint(), s[1]))
            if round == 1 { return soakRound(2) }
            finishSoak()
        default:
            break
        }
    }

    // One soak round: 100k sync calls, then a 100k-message burst.
    private func soakRound(_ round: Int) {
        for _ in 0..<soakCalls { _ = try? bridge.call(Data("echo:soak".utf8)) }
        startBurst(next: .soak(round: round))
    }

    private func finishSoak() {
        let (a, b) = (soakMem[0], soakMem[1])
        log("soak: 2 rounds × (\(soakCalls) calls + \(burstCount) msgs)")
        log("  Go heap \(a.heap / 1024) → \(b.heap / 1024) KB, footprint \(a.footprint / 1024 / 1024) → \(b.footprint / 1024 / 1024) MB, goroutines \(a.goroutines) → \(b.goroutines)")
        let leak = b.heap - a.heap > 512 * 1024 || b.footprint - a.footprint > 4 * 1024 * 1024 || b.goroutines > a.goroutines + 2
        if leak { return fail("memory grew between soak rounds") }
        log("soak: no leak")
        soakOK = true
        stage = .done
        maybePass()
    }

    private func maybePass() {
        if soakOK && ticks == 3 { log("NN-0B RESULT: PASS") }
    }

    // MARK: helpers

    private func text(_ req: String) throws -> String { String(decoding: try bridge.call(Data(req.utf8)), as: UTF8.self) }

    /// heap, goroutines, sent, wakes, drains
    private func stats(gc: Bool) -> [Int] {
        ((try? text(gc ? "stats:gc" : "stats:")) ?? "").split(separator: " ").compactMap { Int($0) }
    }

    private func footprint() -> Int {
        var info = task_vm_info_data_t()
        var count = mach_msg_type_number_t(MemoryLayout<task_vm_info_data_t>.size / MemoryLayout<integer_t>.size)
        let kr = withUnsafeMutablePointer(to: &info) {
            $0.withMemoryRebound(to: integer_t.self, capacity: Int(count)) { task_info(mach_task_self_, task_flavor_t(TASK_VM_INFO), $0, &count) }
        }
        return kr == KERN_SUCCESS ? Int(info.phys_footprint) : 0
    }

    private func fail(_ s: String) { stage = .done; log("FAIL: " + s) }

    private func log(_ s: String) {
        print("[hello] " + s)
        lines.append(s)
        label.text = "Nimbus Native · bridge checks\n\n" + lines.joined(separator: "\n")
    }

    private func us(since t: DispatchTime) -> Double { Double(DispatchTime.now().uptimeNanoseconds - t.uptimeNanoseconds) / 1_000 }
    private func ms(since t: DispatchTime) -> Double { us(since: t) / 1_000 }
    private func fmt(_ v: Double) -> String { String(format: "%.2f", v) }
}

UIApplicationMain(CommandLine.argc, CommandLine.unsafeArgv, nil, NSStringFromClass(AppDelegate.self))
