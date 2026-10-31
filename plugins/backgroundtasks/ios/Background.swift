import BackgroundTasks
import Foundation

/// backgroundtasks: one system task runs whichever of the app's tasks are
/// due. iOS launches the app for it, which starts Go as usual.
enum BackgroundPlugin {
    private static let id = "space.nimbusgo.background"
    private static let key = "nimbus.background.tasks" // name -> [seconds, last run, 1 if once]
    private static var running: Set<String> = []
    private static var current: BGTask?

    private static var tasks: [String: [Double]] {
        get { UserDefaults.standard.dictionary(forKey: key) as? [String: [Double]] ?? [:] }
        set { UserDefaults.standard.set(newValue, forKey: key) }
    }

    static func boot() {
        BGTaskScheduler.shared.register(forTaskWithIdentifier: id, using: .main) { run($0) }
    }

    private static func submit() {
        guard let soonest = tasks.values.map({ $0[0] }).min() else { return BGTaskScheduler.shared.cancelAllTaskRequests() }
        let r = BGAppRefreshTaskRequest(identifier: id)
        r.earliestBeginDate = Date(timeIntervalSinceNow: soonest)
        try? BGTaskScheduler.shared.submit(r)
    }

    private static func run(_ t: BGTask) {
        submit()
        let now = Date().timeIntervalSince1970
        var all = tasks
        let due = all.filter { now - $0.value[1] >= $0.value[0] }.keys
        guard !due.isEmpty else { return t.setTaskCompleted(success: true) }
        current = t
        running = Set(due)
        t.expirationHandler = {
            current?.setTaskCompleted(success: false)
            current = nil
        }
        for n in due {
            if all[n]?.count == 3 { all[n] = nil } else { all[n]?[1] = now }
            try? Bridge.shared.send(Data(Render.Message.backgroundRun(Render.BackgroundRun(name: n)).encode()))
        }
        tasks = all
    }

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "schedule":
            guard let n = call.string("name") else { return call.reject(Plugins.failed, "a task needs a name") }
            tasks[n] = [call.args["seconds"] as? Double ?? 3600, Date().timeIntervalSince1970]
            submit()
            call.resolve()
        case "once":
            guard let n = call.string("name") else { return call.reject(Plugins.failed, "a task needs a name") }
            tasks[n] = [call.args["seconds"] as? Double ?? 0, Date().timeIntervalSince1970, 1]
            submit()
            call.resolve()
        case "cancel":
            if let n = call.string("name") { tasks[n] = nil }
            submit()
            call.resolve()
        case "done":
            if let n = call.string("name") { running.remove(n) }
            if running.isEmpty {
                current?.setTaskCompleted(success: call.args["ok"] as? Bool ?? false)
                current = nil
            }
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
