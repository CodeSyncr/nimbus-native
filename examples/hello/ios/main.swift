import UIKit

/// Bridge and wire checks, run at launch. Everything crosses as generated
/// Proto messages. Every line also goes to the console so
/// `nimbus native run ios` can read the result.
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?
    private let label = UILabel()
    private var lines: [String] = []
    private let bridge = Bridge.shared

    private enum Stage { case ping, burst, soak(round: Int), done }
    private var stage = Stage.ping
    private var t0 = DispatchTime.now()
    private var ticks = 0, pings: UInt32 = 0, burstNext: UInt32 = 0, drainsBefore: UInt64 = 0
    private var soakOK = false
    private var soakMem: [(heap: UInt64, footprint: Int, goroutines: UInt32)] = []

    private let pingCount: UInt32 = 1000, burstCount: UInt32 = 100_000, soakCalls = 100_000

    func application(_ app: UIApplication, didFinishLaunchingWithOptions opts: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        let vc = UIViewController()
        vc.view.backgroundColor = .systemBackground
        label.numberOfLines = 0
        label.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
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

        bridge.onReceive = { [weak self] data in
            do {
                self?.received(try Proto.Message.decode([UInt8](data)))
            } catch {
                self?.fail("undecodable message from Go: \(error)")
            }
        }
        do {
            try run()
        } catch {
            fail("\(error)")
        }
        return true
    }

    private func run() throws {
        do {
            try bridge.start(schema: NimbusSchema.combined ^ 1)
            return fail("a stale schema was accepted")
        } catch let e as BridgeError where e.code == NN_ERR_SCHEMA {
            log("schema guard: stale hash refused")
        }

        t0 = .now()
        try bridge.start(schema: NimbusSchema.combined)
        log("start: ok in \(fmt(ms(since: t0))) ms, schema \(String(NimbusSchema.combined, radix: 16))")

        guard case .greeting(let g) = try call(.hello(Proto.Hello(name: "Yash"))) else { return fail("no greeting") }
        log("call: \(g.text) \(g.goVersion) on \(g.platform)")

        let n = 10_000, b0 = DispatchTime.now()
        let echo = Proto.Message.echo(Proto.Echo(text: "x"))
        for _ in 0..<n { _ = try call(echo) }
        log("typed sync calls: \(n), avg \(fmt(us(since: b0) / Double(n))) µs (encode + call + decode)")

        do {
            _ = try call(.crash(Proto.Crash(reason: "asked to panic")))
            return fail("panic not reported")
        } catch let e as BridgeError where e.code == NN_ERR_PANIC {
            log("panic: recovered, message + Go stack \(e.detail.contains("asked to panic") && e.detail.contains("goroutine") ? "delivered" : "MISSING")")
        }

        var rows = 0
        let s0 = DispatchTime.now()
        for _ in 0..<20 {
            guard case .snapshot(let s) = try call(.snapshotRequest(Proto.SnapshotRequest(rows: 1000))) else { return fail("no snapshot") }
            rows += s.rows.count
            if s.rows[2].subtitle == nil || s.rows[1].subtitle != nil || s.rows[999].attrs["city"] != "Pune" || s.rows[5].kind != .button {
                return fail("snapshot fields wrong")
            }
        }
        log("snapshot: \(rows) rows decoded, \(fmt(us(since: s0) / Double(rows))) µs/row (Go encode + copy + Swift decode)")

        t0 = .now()
        try send(.ping(Proto.Ping(seq: 0)))
    }

    private func received(_ m: Proto.Message) {
        switch (stage, m) {
        case (_, .ready(let r)):
            log("go → native: ready (\(r.goVersion))")
        case (_, .tick(let t)):
            ticks += 1
            log("go → native: tick \(t.n) (goroutine)")
            maybePass()
        case (.ping, .pong(let p)):
            guard p.seq == pings else { return fail("pong \(p.seq), want \(pings)") }
            pings += 1
            if pings < pingCount {
                try? send(.ping(Proto.Ping(seq: pings)))
            } else {
                log("ping-pong: \(pingCount) typed round trips, avg \(fmt(us(since: t0) / Double(pingCount))) µs")
                startBurst(next: .burst)
            }
        case (.burst, .item(let it)), (.soak, .item(let it)):
            guard it.index == burstNext else { return fail("burst out of order: \(it.index), want \(burstNext)") }
            burstNext += 1
            if burstNext == burstCount { burstDone() }
        default:
            fail("unexpected message \(m)")
        }
    }

    private func startBurst(next: Stage) {
        stage = next
        burstNext = 0
        drainsBefore = stats(gc: false).drains
        t0 = .now()
        _ = try? call(.startBurst(Proto.StartBurst(count: burstCount)))
    }

    private func burstDone() {
        let drains = stats(gc: false).drains - drainsBefore
        switch stage {
        case .burst:
            log("burst: \(burstCount) typed msgs in order, \(drains) drains, \(fmt(us(since: t0) * 1000 / Double(burstCount))) ns/msg")
            soakRound(1)
        case .soak(let round):
            let s = stats(gc: true)
            soakMem.append((s.heap, footprint(), s.goroutines))
            if round == 1 { return soakRound(2) }
            finishSoak()
        default:
            break
        }
    }

    private func soakRound(_ round: Int) {
        let echo = Proto.Message.echo(Proto.Echo(text: "soak"))
        for _ in 0..<soakCalls { _ = try? call(echo) }
        startBurst(next: .soak(round: round))
    }

    private func finishSoak() {
        let (a, b) = (soakMem[0], soakMem[1])
        log("soak: 2 rounds × (\(soakCalls) calls + \(burstCount) msgs)")
        log("  Go heap \(a.heap / 1024) → \(b.heap / 1024) KB, footprint \(a.footprint >> 20) → \(b.footprint >> 20) MB, goroutines \(a.goroutines) → \(b.goroutines)")
        if Int(b.heap) - Int(a.heap) > 512 * 1024 || b.footprint - a.footprint > 4 << 20 || b.goroutines > a.goroutines + 2 {
            return fail("memory grew between soak rounds")
        }
        log("soak: no leak")
        soakOK = true
        stage = .done
        maybePass()
    }

    private func maybePass() {
        if soakOK && ticks == 3 { log("NN-0C RESULT: PASS") }
    }

    // MARK: helpers

    private func call(_ m: Proto.Message) throws -> Proto.Message {
        try Proto.Message.decode([UInt8](try bridge.call(Data(m.encode()))))
    }

    private func send(_ m: Proto.Message) throws { try bridge.send(Data(m.encode())) }

    private func stats(gc: Bool) -> Proto.Stats {
        if case .stats(let s)? = try? call(.statsRequest(Proto.StatsRequest(gc: gc))) { return s }
        return Proto.Stats()
    }

    private func footprint() -> Int {
        var info = task_vm_info_data_t()
        var count = mach_msg_type_number_t(MemoryLayout<task_vm_info_data_t>.size / MemoryLayout<integer_t>.size)
        let kr = withUnsafeMutablePointer(to: &info) {
            $0.withMemoryRebound(to: integer_t.self, capacity: Int(count)) { task_info(mach_task_self_, task_flavor_t(TASK_VM_INFO), $0, &count) }
        }
        return kr == KERN_SUCCESS ? Int(info.phys_footprint) : 0
    }

    private func fail(_ s: String) {
        stage = .done
        log("FAIL: " + s)
    }

    private func log(_ s: String) {
        print("[hello] " + s)
        lines.append(s)
        label.text = "Nimbus Native · bridge + wire checks\n\n" + lines.joined(separator: "\n")
    }

    private func us(since t: DispatchTime) -> Double { Double(DispatchTime.now().uptimeNanoseconds - t.uptimeNanoseconds) / 1_000 }
    private func ms(since t: DispatchTime) -> Double { us(since: t) / 1_000 }
    private func fmt(_ v: Double) -> String { String(format: "%.2f", v) }
}

UIApplicationMain(CommandLine.argc, CommandLine.unsafeArgv, nil, NSStringFromClass(AppDelegate.self))
