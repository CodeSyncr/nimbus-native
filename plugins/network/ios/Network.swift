import Foundation
import Network

/// network: the phone's connection, from a path monitor started when first
/// asked. Its first answer takes a moment, and callers wait for it.
enum NetworkPlugin {
    private static let monitor = NWPathMonitor()
    private static var path: NWPath?
    private static var waiting: [PluginCall] = []

    static func handle(_ call: PluginCall) {
        guard call.method == "status" else { return Plugins.unknownMethod(call) }
        if let path { return answer(call, path) }
        waiting.append(call)
        guard waiting.count == 1 else { return }
        monitor.pathUpdateHandler = { p in
            DispatchQueue.main.async {
                path = p
                waiting.forEach { answer($0, p) }
                waiting = []
            }
        }
        monitor.start(queue: DispatchQueue(label: "space.nimbusgo.network"))
    }

    private static func answer(_ call: PluginCall, _ p: NWPath) {
        let on = p.status == .satisfied
        var type = "none"
        if on {
            type = p.usesInterfaceType(.wifi) ? "wifi" : p.usesInterfaceType(.cellular) ? "cellular"
                : p.usesInterfaceType(.wiredEthernet) ? "wired" : "other"
        }
        call.resolve(["connected": on, "type": type, "expensive": p.isExpensive, "address": address(), "airplane": false])
    }

    /// The phone's IPv4 address: Wi-Fi first, then mobile data.
    private static func address() -> String {
        var list: UnsafeMutablePointer<ifaddrs>?
        guard getifaddrs(&list) == 0 else { return "" }
        defer { freeifaddrs(list) }
        var found: [String: String] = [:]
        var at = list
        while let i = at {
            at = i.pointee.ifa_next
            guard let a = i.pointee.ifa_addr, a.pointee.sa_family == UInt8(AF_INET), i.pointee.ifa_flags & UInt32(IFF_UP) != 0 else { continue }
            var host = [CChar](repeating: 0, count: Int(NI_MAXHOST))
            if getnameinfo(a, socklen_t(a.pointee.sa_len), &host, socklen_t(host.count), nil, 0, NI_NUMERICHOST) == 0 {
                found[String(cString: i.pointee.ifa_name)] = String(cString: host)
            }
        }
        return found["en0"] ?? found["pdp_ip0"] ?? found.first { $0.key != "lo0" }?.value ?? ""
    }
}
