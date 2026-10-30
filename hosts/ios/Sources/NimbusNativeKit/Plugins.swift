import CoreHaptics
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
        "dialog": Dialogs.handle,
        "haptics": Haptics.handle,
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

/// dialog: alerts, action sheets, toasts and date pickers, shown over
/// whatever is on screen.
enum Dialogs {
    static func handle(_ call: PluginCall) {
        guard let top = topController() else { return call.reject(Plugins.unavailable, "no window to show a dialog in") }
        switch call.method {
        case "show": show(call, from: top)
        case "toast": toast(call.string("message") ?? "", in: top.view.window ?? top.view)
        case "date": date(call, from: top)
        default: Plugins.unknownMethod(call)
        }
        if call.method == "toast" { call.resolve() }
    }

    static func topController() -> UIViewController? {
        let window = UIApplication.shared.connectedScenes
            .compactMap { ($0 as? UIWindowScene)?.windows.first { $0.isKeyWindow } }.first
        var top = window?.rootViewController
        while let presented = top?.presentedViewController, !presented.isBeingDismissed { top = presented }
        return top
    }

    private static func show(_ call: PluginCall, from top: UIViewController) {
        let sheet = call.args["sheet"] as? Bool ?? false
        let a = UIAlertController(title: call.string("title"), message: call.string("message"),
                                  preferredStyle: sheet ? .actionSheet : .alert)
        let buttons = call.args["buttons"] as? [[String: Any]] ?? [["label": "OK"]]
        for (i, b) in buttons.enumerated() {
            let style: UIAlertAction.Style
            switch b["style"] as? String {
            case "cancel": style = .cancel
            case "destructive": style = .destructive
            default: style = .default
            }
            a.addAction(UIAlertAction(title: b["label"] as? String ?? "", style: style) { _ in call.resolve(i) })
        }
        if let pop = a.popoverPresentationController { // iPad sheets point somewhere
            pop.sourceView = top.view
            pop.sourceRect = CGRect(x: top.view.bounds.midX, y: top.view.bounds.maxY - 40, width: 0, height: 0)
        }
        top.present(a, animated: true)
    }

    private static func toast(_ message: String, in host: UIView) {
        let label = PaddedLabel()
        label.text = message
        label.numberOfLines = 0
        label.textColor = .white
        label.font = .systemFont(ofSize: 15, weight: .medium)
        label.backgroundColor = UIColor(white: 0.1, alpha: 0.9)
        label.layer.cornerRadius = 12
        label.clipsToBounds = true
        label.textAlignment = .center
        let maxW = host.bounds.width - 48
        let size = label.sizeThatFits(CGSize(width: maxW, height: .greatestFiniteMagnitude))
        // Above a tab bar, where one would be (49 pt), clear of the home indicator.
        label.frame = CGRect(x: (host.bounds.width - size.width) / 2, y: host.bounds.height - host.safeAreaInsets.bottom - size.height - 72,
                             width: size.width, height: size.height)
        label.alpha = 0
        host.addSubview(label)
        UIView.animate(withDuration: 0.2) { label.alpha = 1 }
        UIView.animate(withDuration: 0.3, delay: 2.2, options: []) { label.alpha = 0 } completion: { _ in label.removeFromSuperview() }
    }

    private final class PaddedLabel: UILabel {
        let inset = UIEdgeInsets(top: 10, left: 16, bottom: 10, right: 16)
        override func drawText(in rect: CGRect) { super.drawText(in: rect.inset(by: inset)) }
        override func sizeThatFits(_ size: CGSize) -> CGSize {
            let s = super.sizeThatFits(CGSize(width: size.width - inset.left - inset.right, height: size.height))
            return CGSize(width: s.width + inset.left + inset.right, height: s.height + inset.top + inset.bottom)
        }
    }

    private static func date(_ call: PluginCall, from top: UIViewController) {
        let picker = UIDatePicker()
        switch call.string("mode") {
        case "time": picker.datePickerMode = .time
        case "datetime": picker.datePickerMode = .dateAndTime
        default: picker.datePickerMode = .date
        }
        picker.preferredDatePickerStyle = .wheels
        if let ms = call.args["initial"] as? Double { picker.date = Date(timeIntervalSince1970: ms / 1000) }
        let vc = DatePickerController(picker: picker) { date in
            if let date { call.resolve(Int64(date.timeIntervalSince1970 * 1000)) } else { call.reject(Plugins.cancelled, "cancelled") }
        }
        top.present(vc, animated: true)
    }

    /// A small sheet with a wheel picker and Cancel / Done.
    private final class DatePickerController: UIViewController, UIAdaptivePresentationControllerDelegate {
        private let picker: UIDatePicker
        private var done: ((Date?) -> Void)?

        init(picker: UIDatePicker, done: @escaping (Date?) -> Void) {
            self.picker = picker
            self.done = done
            super.init(nibName: nil, bundle: nil)
            modalPresentationStyle = .pageSheet
            if let sheet = sheetPresentationController { sheet.detents = [.medium()] }
            presentationController?.delegate = self
        }

        required init?(coder: NSCoder) { fatalError("not used") }

        override func viewDidLoad() {
            super.viewDidLoad()
            view.backgroundColor = .systemBackground
            let bar = UIToolbar()
            bar.items = [
                UIBarButtonItem(systemItem: .cancel, primaryAction: UIAction { [weak self] _ in self?.finish(nil) }),
                UIBarButtonItem(systemItem: .flexibleSpace),
                UIBarButtonItem(systemItem: .done, primaryAction: UIAction { [weak self] _ in self?.finish(self?.picker.date) }),
            ]
            let stack = UIStackView(arrangedSubviews: [bar, picker])
            stack.axis = .vertical
            stack.translatesAutoresizingMaskIntoConstraints = false
            view.addSubview(stack)
            NSLayoutConstraint.activate([
                stack.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
                stack.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                stack.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            ])
        }

        private func finish(_ date: Date?) {
            done?(date)
            done = nil
            dismiss(animated: true)
        }

        func presentationControllerDidDismiss(_ p: UIPresentationController) {
            done?(nil) // swiped away
            done = nil
        }
    }
}

/// haptics: the system's feedback, and patterns played with Core Haptics
/// (taps at their moments where the phone has no haptic engine).
enum Haptics {
    private static var engine: CHHapticEngine? = {
        guard CHHapticEngine.capabilitiesForHardware().supportsHaptics else { return nil }
        let e = try? CHHapticEngine()
        e?.isAutoShutdownEnabled = true
        return e
    }()

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "impact":
            let style: UIImpactFeedbackGenerator.FeedbackStyle = switch call.string("style") {
            case "light": .light
            case "heavy": .heavy
            case "soft": .soft
            case "rigid": .rigid
            default: .medium
            }
            let i = (call.args["intensity"] as? Double).map { CGFloat($0) } ?? 1
            UIImpactFeedbackGenerator(style: style).impactOccurred(intensity: i > 0 ? i : 1)
        case "notify":
            let kind: UINotificationFeedbackGenerator.FeedbackType = switch call.string("kind") {
            case "warning": .warning
            case "error": .error
            default: .success
            }
            UINotificationFeedbackGenerator().notificationOccurred(kind)
        case "selection":
            UISelectionFeedbackGenerator().selectionChanged()
        case "play":
            play(call)
        default:
            return Plugins.unknownMethod(call)
        }
        call.resolve()
    }

    private static func play(_ call: PluginCall) {
        let events = (call.args["events"] as? [[String: Any]]) ?? []
        func num(_ e: [String: Any], _ k: String) -> Double { (e[k] as? Double) ?? Double((e[k] as? Int) ?? 0) }
        guard let engine else {
            // No engine: a firm tap at each moment.
            for e in events {
                DispatchQueue.main.asyncAfter(deadline: .now() + num(e, "T")) {
                    UIImpactFeedbackGenerator(style: num(e, "S") > 0.6 ? .rigid : .heavy).impactOccurred(intensity: max(0.3, CGFloat(num(e, "I"))))
                }
            }
            return
        }
        var list: [CHHapticEvent] = []
        for e in events {
            let params = [CHHapticEventParameter(parameterID: .hapticIntensity, value: Float(num(e, "I"))),
                          CHHapticEventParameter(parameterID: .hapticSharpness, value: Float(num(e, "S")))]
            let d = num(e, "D")
            list.append(d > 0
                ? CHHapticEvent(eventType: .hapticContinuous, parameters: params, relativeTime: num(e, "T"), duration: d)
                : CHHapticEvent(eventType: .hapticTransient, parameters: params, relativeTime: num(e, "T")))
        }
        var curves: [CHHapticParameterCurve] = []
        for c in (call.args["curves"] as? [[String: Any]]) ?? [] {
            let points = ((c["points"] as? [[Double]]) ?? []).compactMap { p in
                p.count == 2 ? CHHapticParameterCurve.ControlPoint(relativeTime: p[0], value: Float(p[1])) : nil
            }
            guard !points.isEmpty else { continue }
            curves.append(CHHapticParameterCurve(parameterID: (c["sharp"] as? Bool) == true ? .hapticSharpnessControl : .hapticIntensityControl,
                                                 controlPoints: points, relativeTime: 0))
        }
        guard let pattern = try? CHHapticPattern(events: list, parameterCurves: curves),
              let player = try? engine.makePlayer(with: pattern) else { return }
        try? engine.start()
        try? player.start(atTime: CHHapticTimeImmediate)
    }
}
