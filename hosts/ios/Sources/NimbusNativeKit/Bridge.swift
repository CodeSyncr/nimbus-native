import Foundation

/// A failed call into the Go runtime. `code` is one of the NN_ERR_* values
/// from nn.h; for a recovered panic, `detail` holds the Go panic and stack.
public struct BridgeError: Error, CustomStringConvertible {
    public let code: Int32
    public let detail: String
    public var description: String { "nimbus-native error \(code)\(detail.isEmpty ? "" : ": \(detail)")" }
}

/// The Swift side of the Nimbus Native C ABI (nn.h).
public final class Bridge {
    public static let shared = Bridge()

    /// Every message Go sends, delivered on the main thread in order.
    public var onReceive: ((Data) -> Void)?

    private init() {}

    /// Boots the app's Go side. Call once, at launch.
    public func start() throws {
        var host = nn_host()
        host.abi = UInt32(NN_ABI_VERSION)
        host.ctx = Unmanaged.passUnretained(self).toOpaque()
        host.wake = { ctx in
            // A Go thread: Go's outbox just stopped being empty. Schedule one
            // drain on main; everything sent before it runs rides along.
            let bridge = Unmanaged<Bridge>.fromOpaque(ctx!).takeUnretainedValue()
            DispatchQueue.main.async { bridge.drain() }
        }
        let rc = nn_start(&host)
        if rc != NN_OK { throw BridgeError(code: rc, detail: "") }
    }

    /// Delivers every message Go has queued, in order. Runs on main.
    private func drain() {
        var out: UnsafeMutablePointer<UInt8>? = nil
        var len: Int32 = 0
        guard nn_drain(&out, &len) == NN_OK, let out else { return }
        defer { nn_free(out) }
        var off = 0
        let end = Int(len)
        while end - off >= 4 {
            let n = Int(UInt32(out[off]) | UInt32(out[off + 1]) << 8 | UInt32(out[off + 2]) << 16 | UInt32(out[off + 3]) << 24)
            off += 4
            guard n <= end - off else { return }
            onReceive?(Data(bytes: out + off, count: n))
            off += n
        }
    }

    /// Native → Go, asynchronous.
    public func send(_ msg: Data) throws {
        let rc = msg.withUnsafeBytes { nn_send($0.bindMemory(to: UInt8.self).baseAddress, Int32($0.count)) }
        if rc != NN_OK { throw BridgeError(code: rc, detail: "") }
    }

    /// Native → Go, synchronous: blocks until Go answers.
    public func call(_ req: Data) throws -> Data {
        var out: UnsafeMutablePointer<UInt8>? = nil
        var outLen: Int32 = 0
        let rc = req.withUnsafeBytes { nn_call($0.bindMemory(to: UInt8.self).baseAddress, Int32($0.count), &out, &outLen) }
        defer { if let out { nn_free(out) } }
        let body = out.map { Data(bytes: $0, count: Int(outLen)) } ?? Data()
        if rc != NN_OK { throw BridgeError(code: rc, detail: String(decoding: body, as: UTF8.self)) }
        return body
    }
}
