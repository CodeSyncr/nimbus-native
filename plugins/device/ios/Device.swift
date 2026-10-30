import UIKit

/// device: the phone's model, system, memory and battery.
enum DevicePlugin {
    static func handle(_ call: PluginCall) {
        switch call.method {
        case "info":
            var sys = utsname()
            uname(&sys)
            var model = withUnsafeBytes(of: &sys.machine) { String(cString: $0.bindMemory(to: CChar.self).baseAddress!) }
            #if targetEnvironment(simulator)
            let virtual = true
            model = ProcessInfo.processInfo.environment["SIMULATOR_MODEL_IDENTIFIER"] ?? model
            #else
            let virtual = false
            #endif
            call.resolve(["platform": "ios", "model": model, "manufacturer": "Apple", "osVersion": UIDevice.current.systemVersion,
                          "memory": ProcessInfo.processInfo.physicalMemory, "virtual": virtual])
        case "battery":
            let d = UIDevice.current
            d.isBatteryMonitoringEnabled = true
            call.resolve(["level": d.batteryLevel, "charging": d.batteryState == .charging || d.batteryState == .full])
        default:
            Plugins.unknownMethod(call)
        }
    }
}
