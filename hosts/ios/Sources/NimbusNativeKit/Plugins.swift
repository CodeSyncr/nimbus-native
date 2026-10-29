import Foundation
import Security
import UIKit

/// One call of a plugin method from Go. Answer it once, with `resolve` or
/// `reject`, from any thread.
public final class PluginCall {
    public let plugin: String
    public let method: String
    /// The arguments when Go passed an object (a struct or map); else empty.
    public let args: [String: Any]
    /// The arguments as Go passed them: any JSON value, or nil.
    public let rawArgs: Any?
    private let id: UInt64
    private var answered = false
    private let lock = NSLock()

    init(id: UInt64, plugin: String, method: String, json: String) {
        self.id = id
        self.plugin = plugin
        self.method = method
        rawArgs = json.isEmpty ? nil : try? JSONSerialization.jsonObject(with: Data(json.utf8), options: [.fragmentsAllowed])
        args = rawArgs as? [String: Any] ?? [:]
    }

    /// A string argument, or nil.
    public func string(_ key: String) -> String? { args[key] as? String }

    /// Answers with any JSON value: nil, Bool, numbers, String, arrays and dictionaries of them.
    public func resolve(_ value: Any? = nil) {
        guard let value, !(value is NSNull) else { return finish(value: "null") }
        guard let data = try? JSONSerialization.data(withJSONObject: value, options: [.fragmentsAllowed]) else {
            return reject(Plugins.failed, "the answer is not JSON: \(value)")
        }
        finish(value: String(decoding: data, as: UTF8.self))
    }

    /// Fails the call. `code` is one of Plugins.cancelled, .denied, .notFound, .unavailable, .failed.
    public func reject(_ code: String, _ message: String) { finish(code: code, err: message) }

    private func finish(value: String = "", code: String = "", err: String = "") {
        lock.lock()
        defer { lock.unlock() }
        guard !answered else { return }
        answered = true
        let msg = Render.Message.pluginResult(Render.PluginResult(id: id, value: value, code: code, err: err))
        try? Bridge.shared.send(Data(msg.encode()))
    }
}

/// Native code Go can call (package plugin). Register app plugins at launch,
/// before the first NimbusViewController:
///
///     Plugins.register("battery") { call in
///         UIDevice.current.isBatteryMonitoringEnabled = true
///         call.resolve(UIDevice.current.batteryLevel)
///     }
///
/// Handlers run on the main thread.
public enum Plugins {
    public typealias Handler = (PluginCall) -> Void

    public static let cancelled = "cancelled", denied = "denied", notFound = "not_found",
                      unavailable = "unavailable", failed = "failed"

    private static let lock = NSLock()
    private static var handlers: [String: Handler] = [
        "secureStorage": SecureStorage.handle,
        "app": AppInfo.handle,
        "linking": Linking.handle,
    ]

    /// Makes `name` callable from Go, replacing any plugin of that name.
    public static func register(_ name: String, _ handler: @escaping Handler) {
        lock.lock()
        handlers[name] = handler
        lock.unlock()
    }

    /// Starts a call from Go (on a Go thread): accepted unless no plugin has the name.
    static func start(_ c: Render.PluginCall) -> Render.PluginAck {
        lock.lock()
        let handler = handlers[c.plugin]
        lock.unlock()
        guard let handler else { return Render.PluginAck(err: "no plugin named \(c.plugin) in this app") }
        let call = PluginCall(id: c.id, plugin: c.plugin, method: c.method, json: c.args)
        DispatchQueue.main.async { handler(call) }
        return Render.PluginAck()
    }

    static func unknownMethod(_ call: PluginCall) {
        call.reject(unavailable, "\(call.plugin) has no method \(call.method)")
    }
}

/// secureStorage: small secrets (tokens, keys) in the Keychain, readable
/// after the first unlock and never synced or backed up off the device.
enum SecureStorage {
    static func handle(_ call: PluginCall) {
        guard let key = call.string("key"), !key.isEmpty else { return call.reject(Plugins.failed, "key is required") }
        var query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: Bundle.main.bundleIdentifier ?? "nimbus-native",
            kSecAttrAccount as String: key,
        ]
        switch call.method {
        case "get":
            query[kSecReturnData as String] = true
            query[kSecMatchLimit as String] = kSecMatchLimitOne
            var out: CFTypeRef?
            let status = SecItemCopyMatching(query as CFDictionary, &out)
            if status == errSecItemNotFound { return call.resolve(["value": NSNull(), "found": false]) }
            guard status == errSecSuccess, let data = out as? Data else { return call.reject(Plugins.failed, "keychain error \(status)") }
            call.resolve(["value": String(decoding: data, as: UTF8.self), "found": true])
        case "set":
            guard let value = call.string("value") else { return call.reject(Plugins.failed, "value is required") }
            SecItemDelete(query as CFDictionary)
            query[kSecValueData as String] = Data(value.utf8)
            query[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
            let status = SecItemAdd(query as CFDictionary, nil)
            status == errSecSuccess ? call.resolve() : call.reject(Plugins.failed, "keychain error \(status)")
        case "delete":
            let status = SecItemDelete(query as CFDictionary)
            status == errSecSuccess || status == errSecItemNotFound ? call.resolve() : call.reject(Plugins.failed, "keychain error \(status)")
        default:
            Plugins.unknownMethod(call)
        }
    }
}

/// app: facts about the app and device, and where it may write files.
enum AppInfo {
    static func handle(_ call: PluginCall) {
        guard call.method == "info" else { return Plugins.unknownMethod(call) }
        let fm = FileManager.default
        let data = fm.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        let cache = fm.urls(for: .cachesDirectory, in: .userDomainMask)[0]
        try? fm.createDirectory(at: data, withIntermediateDirectories: true)
        let info = Bundle.main.infoDictionary ?? [:]
        call.resolve([
            "platform": "ios",
            "id": Bundle.main.bundleIdentifier ?? "",
            "version": info["CFBundleShortVersionString"] as? String ?? "",
            "build": info["CFBundleVersion"] as? String ?? "",
            "osVersion": UIDevice.current.systemVersion,
            "locale": Locale.preferredLanguages.first ?? Locale.current.identifier,
            "dataDir": data.path,
            "cacheDir": cache.path,
        ])
    }
}

/// linking: open URLs in other apps (the browser, Maps, Mail, tel:).
enum Linking {
    static func handle(_ call: PluginCall) {
        guard let s = call.string("url"), let url = URL(string: s) else { return call.reject(Plugins.failed, "url is required") }
        switch call.method {
        case "open":
            UIApplication.shared.open(url) { ok in ok ? call.resolve(true) : call.reject(Plugins.unavailable, "nothing opens \(s)") }
        case "canOpen":
            call.resolve(UIApplication.shared.canOpenURL(url))
        default:
            Plugins.unknownMethod(call)
        }
    }
}
