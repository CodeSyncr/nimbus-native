import Foundation
#if canImport(NimbusNativeC)
import NimbusNativeC // a Swift package build; the CLI build uses a bridging header
#endif

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

    /// Answers Go's synchronous questions (text measurement, native modules).
    /// Called on a Go thread, never the main thread: it must not wait on main.
    /// Set it before start.
    public var onHostCall: ((Data) throws -> Data)?

    private init() {}

    /// Boots the app's Go side. Call once, at launch. `schema` is the
    /// generated `NimbusSchema.combined`; the runtime refuses to start
    /// (NN_ERR_SCHEMA) if the Go side was generated from a different schema.
    private var started = false

    /// Whether this start found the runtime already running (and left it so).
    @discardableResult
    public func start(schema: UInt64) throws -> Bool {
        if started { return false } // a new view controller reattaches instead
        var host = nn_host()
        host.abi = UInt32(NN_ABI_VERSION)
        host.schema = schema
        host.ctx = Unmanaged.passUnretained(self).toOpaque()
        host.wake = { ctx in
            // A Go thread: Go's outbox just stopped being empty. Schedule one
            // drain on main; everything sent before it runs rides along.
            let bridge = Unmanaged<Bridge>.fromOpaque(ctx!).takeUnretainedValue()
            DispatchQueue.main.async { bridge.drain() }
        }
        host.call = { ctx, req, len, out, outLen in
            let bridge = Unmanaged<Bridge>.fromOpaque(ctx!).takeUnretainedValue()
            guard let handler = bridge.onHostCall else { return NN_ERR_NO_HANDLER }
            let request = (req != nil && len > 0) ? Data(bytes: req!, count: Int(len)) : Data()
            guard let reply = try? handler(request) else { return NN_ERR_ARG }
            let p = malloc(max(reply.count, 1))!.assumingMemoryBound(to: UInt8.self)
            reply.copyBytes(to: p, count: reply.count)
            out!.pointee = p
            outLen!.pointee = Int32(reply.count)
            return NN_OK
        }
        let rc = nn_start(&host)
        if rc != NN_OK { throw BridgeError(code: rc, detail: "") }
        started = true
        return true
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
