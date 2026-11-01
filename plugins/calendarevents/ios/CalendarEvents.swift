import EventKit

/// calendarevents: EventKit, with full access.
enum CalendarEventsPlugin {
    private static var store = EKEventStore()

    private static var allowed: Bool {
        let s = EKEventStore.authorizationStatus(for: .event)
        if #available(iOS 17, *) { return s == .fullAccess }
        return s == .authorized
    }

    static func handle(_ call: PluginCall) {
        if call.method == "request" { return request(call) }
        guard allowed else { return call.reject(Plugins.denied, "calendars aren't allowed: call Request first") }
        do {
            switch call.method {
            case "calendars":
                let first = store.defaultCalendarForNewEvents?.calendarIdentifier
                call.resolve(store.calendars(for: .event).map {
                    ["id": $0.calendarIdentifier, "title": $0.title, "writable": $0.allowsContentModifications, "default": $0.calendarIdentifier == first]
                })
            case "newCalendar":
                let c = EKCalendar(for: .event, eventStore: store)
                c.title = call.string("title") ?? "Calendar"
                guard let source = store.defaultCalendarForNewEvents?.source ?? store.sources.first(where: { $0.sourceType == .local }) else {
                    return call.reject(Plugins.unavailable, "this phone has nowhere to keep a calendar")
                }
                c.source = source
                try store.saveCalendar(c, commit: true)
                call.resolve(["id": c.calendarIdentifier])
            case "deleteCalendar":
                guard let c = store.calendar(withIdentifier: call.string("id") ?? "") else { return call.reject(Plugins.notFound, "no such calendar") }
                try store.removeCalendar(c, commit: true)
                call.resolve()
            case "list":
                let from = Date(timeIntervalSince1970: call.args["from"] as? Double ?? 0)
                let to = Date(timeIntervalSince1970: call.args["to"] as? Double ?? 0)
                let found = store.events(matching: store.predicateForEvents(withStart: from, end: to, calendars: nil))
                call.resolve(found.sorted { $0.startDate < $1.startDate }.map(describe))
            case "create":
                let e = EKEvent(eventStore: store)
                try fill(e, call)
                call.resolve(["id": e.eventIdentifier ?? ""])
            case "update":
                guard let e = store.event(withIdentifier: call.string("id") ?? "") else { return call.reject(Plugins.notFound, "no such event") }
                try fill(e, call)
                call.resolve()
            case "delete":
                guard let e = store.event(withIdentifier: call.string("id") ?? "") else { return call.reject(Plugins.notFound, "no such event") }
                try store.remove(e, span: .futureEvents, commit: true)
                call.resolve()
            default:
                Plugins.unknownMethod(call)
            }
        } catch {
            call.reject(Plugins.failed, error.localizedDescription)
        }
    }

    private static func request(_ call: PluginCall) {
        let done: (Bool, Error?) -> Void = { ok, _ in
            store = EKEventStore() // one made before permission sees nothing
            call.resolve(["granted": ok])
        }
        if #available(iOS 17, *) { store.requestFullAccessToEvents(completion: done) } else { store.requestAccess(to: .event, completion: done) }
    }

    private static func fill(_ e: EKEvent, _ call: PluginCall) throws {
        e.title = call.string("title")
        e.location = call.string("location")
        e.notes = call.string("notes")
        e.startDate = Date(timeIntervalSince1970: call.args["start"] as? Double ?? 0)
        e.endDate = Date(timeIntervalSince1970: call.args["end"] as? Double ?? 0)
        e.isAllDay = call.args["allDay"] as? Bool ?? false
        if let id = call.string("calendarId"), !id.isEmpty, let c = store.calendar(withIdentifier: id) {
            e.calendar = c
        } else if e.calendar == nil {
            e.calendar = store.defaultCalendarForNewEvents
        }
        try store.save(e, span: .futureEvents, commit: true)
    }

    private static func describe(_ e: EKEvent) -> [String: Any] {
        ["id": e.eventIdentifier ?? "", "calendarId": e.calendar.calendarIdentifier, "title": e.title ?? "", "location": e.location ?? "",
         "notes": e.notes ?? "", "start": e.startDate.timeIntervalSince1970, "end": e.endDate.timeIntervalSince1970, "allDay": e.isAllDay]
    }
}
