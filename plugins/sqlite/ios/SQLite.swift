import Foundation
import SQLite3

/// sqlite: the SQLite that comes with iOS, through its C API. Each open
/// database has a serial queue, so Go's statements run one at a time and in
/// order, and a transaction holds until COMMIT or ROLLBACK.
enum SQLitePlugin {
    private final class Handle {
        let db: OpaquePointer
        let queue: DispatchQueue
        init(_ db: OpaquePointer, _ id: String) { self.db = db; queue = DispatchQueue(label: "nimbus.sqlite." + id) }
    }

    private static let lock = NSLock()
    private static var open: [String: Handle] = [:]
    private static let transient = unsafeBitCast(-1, to: sqlite3_destructor_type.self)

    private struct Failure: Error { let message: String }

    /// A plain file name, in Application Support/databases.
    private static func path(_ name: String?) -> String? {
        guard let name, !name.isEmpty, !name.contains("/"), name != ".", name != ".." else { return nil }
        let dir = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("databases", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent(name).path
    }

    private static func message(_ db: OpaquePointer) -> String { String(cString: sqlite3_errmsg(db)) }

    private static func prepare(_ db: OpaquePointer, _ call: PluginCall) throws -> OpaquePointer {
        var s: OpaquePointer?
        guard sqlite3_prepare_v2(db, call.string("sql") ?? "", -1, &s, nil) == SQLITE_OK, let s else {
            throw Failure(message: message(db))
        }
        for (n, v) in (call.args["args"] as? [Any] ?? []).enumerated() {
            let at = Int32(n + 1)
            let rc: Int32
            switch v {
            case is NSNull: rc = sqlite3_bind_null(s, at)
            case let t as String: rc = sqlite3_bind_text(s, at, t, -1, transient)
            case let d as [String: Any] where d["i"] is String:
                guard let i = Int64(d["i"] as! String) else { sqlite3_finalize(s); throw Failure(message: "a strange integer for parameter \(at)") }
                rc = sqlite3_bind_int64(s, at, i)
            case let d as [String: Any] where d["b"] is String:
                let b = Data(base64Encoded: d["b"] as! String) ?? Data()
                rc = b.withUnsafeBytes { sqlite3_bind_blob(s, at, $0.baseAddress, Int32(b.count), transient) }
            case let x as NSNumber: rc = sqlite3_bind_double(s, at, x.doubleValue)
            default: sqlite3_finalize(s); throw Failure(message: "a strange value for parameter \(at)")
            }
            if rc != SQLITE_OK {
                let m = message(db)
                sqlite3_finalize(s)
                throw Failure(message: m)
            }
        }
        return s
    }

    private static func exec(_ db: OpaquePointer, _ call: PluginCall) throws -> Any {
        let s = try prepare(db, call)
        defer { sqlite3_finalize(s) }
        var rc = sqlite3_step(s)
        while rc == SQLITE_ROW { rc = sqlite3_step(s) }
        guard rc == SQLITE_DONE else { throw Failure(message: message(db)) }
        return ["changes": Int(sqlite3_changes(db)), "lastId": ["i": String(sqlite3_last_insert_rowid(db))]]
    }

    private static func query(_ db: OpaquePointer, _ call: PluginCall) throws -> Any {
        let s = try prepare(db, call)
        defer { sqlite3_finalize(s) }
        let count = sqlite3_column_count(s)
        let columns = (0..<count).map { String(cString: sqlite3_column_name(s, $0)) }
        var rows: [[Any]] = []
        var rc = sqlite3_step(s)
        while rc == SQLITE_ROW {
            rows.append((0..<count).map { i -> Any in
                switch sqlite3_column_type(s, i) {
                case SQLITE_NULL: return NSNull()
                case SQLITE_INTEGER: return ["i": String(sqlite3_column_int64(s, i))]
                case SQLITE_FLOAT: return sqlite3_column_double(s, i)
                case SQLITE_BLOB:
                    let n = Int(sqlite3_column_bytes(s, i))
                    let d = n == 0 ? Data() : Data(bytes: sqlite3_column_blob(s, i), count: n)
                    return ["b": d.base64EncodedString()]
                default: return String(cString: sqlite3_column_text(s, i))
                }
            })
            rc = sqlite3_step(s)
        }
        guard rc == SQLITE_DONE else { throw Failure(message: message(db)) }
        return ["columns": columns, "rows": rows]
    }

    /// Runs on the database's queue and answers from there.
    private static func on(_ call: PluginCall, _ work: @escaping (OpaquePointer) throws -> Any) {
        lock.lock()
        let h = open[call.string("id") ?? ""]
        lock.unlock()
        guard let h else { return call.reject(Plugins.notFound, "that database is closed") }
        h.queue.async {
            do { call.resolve(try work(h.db)) } catch let f as Failure { call.reject(Plugins.failed, f.message) } catch {
                call.reject(Plugins.failed, error.localizedDescription)
            }
        }
    }

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "open":
            let name = call.string("name")
            guard let file = name == ":memory:" ? ":memory:" : path(name) else {
                return call.reject(Plugins.failed, "a database name is a plain file name")
            }
            var db: OpaquePointer?
            let rc = sqlite3_open_v2(file, &db, SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE | SQLITE_OPEN_FULLMUTEX, nil)
            guard rc == SQLITE_OK, let db else {
                let m = db.map(message) ?? "can't open the database"
                sqlite3_close(db)
                return call.reject(Plugins.failed, m)
            }
            let id = UUID().uuidString
            lock.lock()
            open[id] = Handle(db, id)
            lock.unlock()
            call.resolve(["id": id])
        case "exec": on(call) { try exec($0, call) }
        case "query": on(call) { try query($0, call) }
        case "close":
            lock.lock()
            let h = open.removeValue(forKey: call.string("id") ?? "")
            lock.unlock()
            guard let h else { return call.resolve() }
            h.queue.async {
                sqlite3_close_v2(h.db)
                call.resolve()
            }
        case "delete":
            guard let p = path(call.string("name")) else { return call.reject(Plugins.failed, "a database name is a plain file name") }
            for suffix in ["", "-wal", "-shm", "-journal"] { try? FileManager.default.removeItem(atPath: p + suffix) }
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
