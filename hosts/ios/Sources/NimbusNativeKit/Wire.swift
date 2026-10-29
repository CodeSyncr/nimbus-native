// The Swift runtime for the Nimbus Native wire format (see wire/wire.go for
// the encoding). Generated code calls these; it uses only the standard
// library so it also builds for the desktop conformance test.

public enum WireError: Error, Equatable {
    case short      // message ends early
    case overflow   // varint overflows 64 bits
    case range      // value out of range for its type
    case variant    // unknown union variant or message id
    case trailing   // bytes left over after the message
}

public struct WireWriter {
    public private(set) var bytes: [UInt8] = []

    public init() {}

    public mutating func bool(_ v: Bool) { bytes.append(v ? 1 : 0) }

    public mutating func uint(_ v: UInt64) {
        var v = v
        while v >= 0x80 {
            bytes.append(UInt8(truncatingIfNeeded: v) | 0x80)
            v >>= 7
        }
        bytes.append(UInt8(v))
    }

    public mutating func int(_ v: Int64) { uint(UInt64(bitPattern: (v << 1) ^ (v >> 63))) }

    public mutating func float32(_ v: Float) { fixed(UInt64(v.bitPattern), 4) }

    public mutating func float64(_ v: Double) { fixed(v.bitPattern, 8) }

    public mutating func string(_ v: String) { bytes(Array(v.utf8)) }

    public mutating func bytes(_ v: [UInt8]) {
        uint(UInt64(v.count))
        bytes.append(contentsOf: v)
    }

    private mutating func fixed(_ v: UInt64, _ n: Int) {
        for i in 0..<n { bytes.append(UInt8(truncatingIfNeeded: v >> (8 * UInt64(i)))) }
    }

    /// Map keys go on the wire sorted: strings by their UTF-8 bytes (Go's
    /// order), integers numerically.
    public static func sortedKeys<V>(_ m: [String: V]) -> [String] {
        m.keys.sorted { $0.utf8.lexicographicallyPrecedes($1.utf8) }
    }

    public static func sortedKeys<K: FixedWidthInteger, V>(_ m: [K: V]) -> [K] { m.keys.sorted() }
}

public struct WireReader {
    private let b: [UInt8]
    private var pos = 0

    public init(_ bytes: [UInt8]) { b = bytes }

    private var remaining: Int { b.count - pos }

    public mutating func bool() throws -> Bool {
        guard pos < b.count else { throw WireError.short }
        let v = b[pos]
        pos += 1
        guard v <= 1 else { throw WireError.range }
        return v == 1
    }

    /// Unsigned LEB128, with the same overflow rule as Go's binary.Uvarint.
    public mutating func uint() throws -> UInt64 {
        var x: UInt64 = 0
        var shift: UInt64 = 0
        for i in 0..<10 {
            guard pos < b.count else { throw WireError.short }
            let c = b[pos]
            pos += 1
            if c < 0x80 {
                if i == 9 && c > 1 { throw WireError.overflow }
                return x | UInt64(c) << shift
            }
            x |= UInt64(c & 0x7f) << shift
            shift += 7
        }
        throw WireError.overflow
    }

    public mutating func int() throws -> Int64 {
        let u = try uint()
        return Int64(bitPattern: (u >> 1) ^ (0 &- (u & 1)))
    }

    public mutating func uint(max: UInt64) throws -> UInt64 {
        let v = try uint()
        guard v <= max else { throw WireError.range }
        return v
    }

    public mutating func int(min: Int64, max: Int64) throws -> Int64 {
        let v = try int()
        guard v >= min && v <= max else { throw WireError.range }
        return v
    }

    public mutating func float32() throws -> Float { Float(bitPattern: UInt32(try fixed(4))) }

    public mutating func float64() throws -> Double { Double(bitPattern: try fixed(8)) }

    public mutating func string() throws -> String { String(decoding: try bytes(), as: UTF8.self) }

    public mutating func bytes() throws -> [UInt8] {
        let n = try uint()
        guard n <= UInt64(remaining) else { throw WireError.short }
        let out = Array(b[pos..<pos + Int(n)])
        pos += Int(n)
        return out
    }

    /// A list or map length. Every element takes at least one byte, so a
    /// count beyond what is left is corrupt.
    public mutating func count() throws -> Int {
        let n = try uint()
        guard n <= UInt64(remaining) else { throw WireError.short }
        return Int(n)
    }

    public mutating func list<T>(_ element: (inout WireReader) throws -> T) throws -> [T] {
        let n = try count()
        var out: [T] = []
        out.reserveCapacity(Swift.min(n, 4096))
        for _ in 0..<n { out.append(try element(&self)) }
        return out
    }

    public mutating func map<K: Hashable, V>(_ key: (inout WireReader) throws -> K,
                                             _ value: (inout WireReader) throws -> V) throws -> [K: V] {
        let n = try count()
        var out: [K: V] = [:]
        out.reserveCapacity(Swift.min(n, 4096))
        for _ in 0..<n {
            let k = try key(&self)
            out[k] = try value(&self)
        }
        return out
    }

    /// Fails with .trailing if bytes remain.
    public func done() throws {
        if pos != b.count { throw WireError.trailing }
    }

    private mutating func fixed(_ n: Int) throws -> UInt64 {
        guard remaining >= n else { throw WireError.short }
        var v: UInt64 = 0
        for i in 0..<n { v |= UInt64(b[pos + i]) << (8 * UInt64(i)) }
        pos += n
        return v
    }
}
