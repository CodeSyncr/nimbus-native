import EventKit
import EventKitUI
import UIKit

/// calendar: the system's event editor.
final class CalendarPlugin: NSObject, EKEventEditViewDelegate {
    private static let shared = CalendarPlugin()
    private var call: PluginCall?

    static func handle(_ c: PluginCall) {
        guard c.method == "add" else { return Plugins.unknownMethod(c) }
        guard let top = NimbusViewController.top else { return c.reject(Plugins.failed, "no screen to open it from") }
        let store = EKEventStore()
        let e = EKEvent(eventStore: store)
        e.title = c.string("title")
        e.location = c.string("location")
        e.notes = c.string("notes")
        e.startDate = Date(timeIntervalSince1970: c.args["start"] as? Double ?? 0)
        e.endDate = Date(timeIntervalSince1970: c.args["end"] as? Double ?? 0)
        let v = EKEventEditViewController()
        v.eventStore = store
        v.event = e
        v.editViewDelegate = shared
        shared.call = c
        top.present(v, animated: true)
    }

    func eventEditViewController(_ v: EKEventEditViewController, didCompleteWith action: EKEventEditViewAction) {
        v.dismiss(animated: true)
        call?.resolve(["saved": action == .saved])
        call = nil
    }
}
