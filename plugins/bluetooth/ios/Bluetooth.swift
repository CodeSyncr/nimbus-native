import CoreBluetooth

/// bluetooth: Core Bluetooth as a central. One operation at a time per kind.
final class BluetoothPlugin: NSObject, CBCentralManagerDelegate, CBPeripheralDelegate {
    private static let shared = BluetoothPlugin()
    private lazy var central = CBCentralManager(delegate: self, queue: .main)
    private var found: [UUID: CBPeripheral] = [:]
    private var inbox: [[String: Any]] = []
    private var asking: PluginCall?
    private var connecting: [UUID: PluginCall] = [:]
    private var pending: [String: PluginCall] = [:]
    private var scanning: [CBUUID]?

    static func handle(_ c: PluginCall) { shared.handle(c) }

    private func characteristic(_ c: PluginCall) -> (CBPeripheral, CBCharacteristic)? {
        guard let id = c.string("id").flatMap(UUID.init(uuidString:)), let p = found[id],
              let s = p.services?.first(where: { $0.uuid == CBUUID(string: c.string("service") ?? "") }),
              let x = s.characteristics?.first(where: { $0.uuid == CBUUID(string: c.string("characteristic") ?? "") }) else { return nil }
        return (p, x)
    }

    private func handle(_ c: PluginCall) {
        switch c.method {
        case "request":
            if central.state != .unknown { return c.resolve(["granted": central.state == .poweredOn]) }
            asking = c
        case "scanStart":
            guard central.state == .poweredOn else { return c.reject(Plugins.unavailable, "Bluetooth is off or not allowed") }
            scanning = (c.args["services"] as? [String] ?? []).map { CBUUID(string: $0) }
            central.scanForPeripherals(withServices: scanning?.isEmpty == true ? nil : scanning)
            c.resolve()
        case "scanRead":
            c.resolve(["devices": inbox])
            inbox = []
        case "scanStop":
            central.stopScan()
            c.resolve()
        case "connect":
            guard let id = c.string("id").flatMap(UUID.init(uuidString:)), let p = found[id] else { return c.reject(Plugins.notFound, "scan for it first") }
            connecting[id] = c
            p.delegate = self
            central.connect(p)
        case "disconnect":
            if let id = c.string("id").flatMap(UUID.init(uuidString:)), let p = found[id] { central.cancelPeripheralConnection(p) }
            c.resolve()
        case "read":
            guard let (p, x) = characteristic(c) else { return c.reject(Plugins.notFound, "no such characteristic") }
            pending["r" + x.uuid.uuidString] = c
            p.readValue(for: x)
        case "write":
            guard let (p, x) = characteristic(c), let d = c.string("value").flatMap({ Data(base64Encoded: $0) }) else { return c.reject(Plugins.notFound, "no such characteristic") }
            pending["w" + x.uuid.uuidString] = c
            p.writeValue(d, for: x, type: .withResponse)
        default:
            Plugins.unknownMethod(c)
        }
    }

    func centralManagerDidUpdateState(_ m: CBCentralManager) {
        guard m.state != .unknown, let c = asking else { return }
        asking = nil
        c.resolve(["granted": m.state == .poweredOn])
    }

    func centralManager(_ m: CBCentralManager, didDiscover p: CBPeripheral, advertisementData a: [String: Any], rssi: NSNumber) {
        found[p.identifier] = p
        inbox.append(["id": p.identifier.uuidString, "name": p.name ?? "", "signal": rssi.intValue])
    }

    func centralManager(_ m: CBCentralManager, didConnect p: CBPeripheral) { p.discoverServices(nil) }

    func centralManager(_ m: CBCentralManager, didFailToConnect p: CBPeripheral, error: Error?) {
        connecting.removeValue(forKey: p.identifier)?.reject(Plugins.failed, error?.localizedDescription ?? "couldn't connect")
    }

    func peripheral(_ p: CBPeripheral, didDiscoverServices error: Error?) {
        guard let services = p.services, !services.isEmpty else {
            connecting.removeValue(forKey: p.identifier)?.resolve()
            return
        }
        services.forEach { p.discoverCharacteristics(nil, for: $0) }
    }

    func peripheral(_ p: CBPeripheral, didDiscoverCharacteristicsFor s: CBService, error: Error?) {
        if p.services?.allSatisfy({ $0.characteristics != nil }) == true { connecting.removeValue(forKey: p.identifier)?.resolve() }
    }

    func peripheral(_ p: CBPeripheral, didUpdateValueFor x: CBCharacteristic, error: Error?) {
        guard let c = pending.removeValue(forKey: "r" + x.uuid.uuidString) else { return }
        if let error { c.reject(Plugins.failed, error.localizedDescription) } else { c.resolve(["value": (x.value ?? Data()).base64EncodedString()]) }
    }

    func peripheral(_ p: CBPeripheral, didWriteValueFor x: CBCharacteristic, error: Error?) {
        guard let c = pending.removeValue(forKey: "w" + x.uuid.uuidString) else { return }
        if let error { c.reject(Plugins.failed, error.localizedDescription) } else { c.resolve() }
    }
}
