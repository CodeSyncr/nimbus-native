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
        host.receive = { ctx, data, len in
            // Any Go thread: copy now (the buffer is borrowed), deliver on main.
            let bridge = Unmanaged<Bridge>.fromOpaque(ctx!).takeUnretainedValue()
            let msg = (data != nil && len > 0) ? Data(bytes: data!, count: Int(len)) : Data()
            DispatchQueue.main.async { bridge.onReceive?(msg) }
        }
        let rc = nn_start(&host)
        if rc != NN_OK { throw BridgeError(code: rc, detail: "") }
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
