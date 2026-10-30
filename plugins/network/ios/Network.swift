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
        call.resolve(["connected": on, "type": type, "expensive": p.isExpensive])
    }
}
