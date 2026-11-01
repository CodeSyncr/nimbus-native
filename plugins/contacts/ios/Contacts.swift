import Contacts
import ContactsUI
import UIKit

/// contacts: the system picker, and the store with permission.
final class ContactsPlugin: NSObject, CNContactPickerDelegate, CNContactViewControllerDelegate {
    private static let shared = ContactsPlugin()
    private var call: PluginCall?

    static func handle(_ c: PluginCall) { shared.handle(c) }

    /// Someone who shared only some contacts has allowed it, for those.
    private static var allowed: Bool {
        let s = CNContactStore.authorizationStatus(for: .contacts)
        if #available(iOS 18, *), s == .limited { return true }
        return s == .authorized
    }

    private static func dict(_ c: CNContact) -> [String: Any] {
        ["id": c.identifier, "name": CNContactFormatter.string(from: c, style: .fullName) ?? "",
         "phones": c.phoneNumbers.map { $0.value.stringValue },
         "emails": c.emailAddresses.map { $0.value as String }]
    }

    private func handle(_ c: PluginCall) {
        let store = CNContactStore()
        switch c.method {
        case "pick":
            guard let top = NimbusViewController.top else { return c.reject(Plugins.failed, "no screen to open it from") }
            call = c
            let p = CNContactPickerViewController()
            p.delegate = self
            top.present(p, animated: true)
        case "request":
            store.requestAccess(for: .contacts) { ok, _ in c.resolve(["granted": ok]) }
        case "list":
            guard Self.allowed else { return c.reject(Plugins.denied, "contacts are not allowed") }
            DispatchQueue.global(qos: .userInitiated).async {
                let keys: [CNKeyDescriptor] = [CNContactFormatter.descriptorForRequiredKeys(for: .fullName),
                                               CNContactPhoneNumbersKey as CNKeyDescriptor, CNContactEmailAddressesKey as CNKeyDescriptor]
                var out: [[String: Any]] = []
                try? store.enumerateContacts(with: CNContactFetchRequest(keysToFetch: keys)) { x, _ in out.append(Self.dict(x)) }
                c.resolve(["contacts": out])
            }
        case "add":
            guard let top = NimbusViewController.top else { return c.reject(Plugins.failed, "no screen to open it from") }
            call = c
            let new = CNMutableContact()
            Self.fill(new, c)
            let v = CNContactViewController(forNewContact: new)
            v.delegate = self
            top.present(UINavigationController(rootViewController: v), animated: true)
        case "create", "update", "delete":
            guard Self.allowed else { return c.reject(Plugins.denied, "contacts aren't allowed: call Request first") }
            DispatchQueue.global(qos: .userInitiated).async { Self.write(c, store) }
        default:
            Plugins.unknownMethod(c)
        }
    }

    private static func fill(_ n: CNMutableContact, _ c: PluginCall) {
        let words = (c.string("name") ?? "").split(separator: " ", maxSplits: 1).map(String.init)
        n.givenName = words.first ?? ""
        n.familyName = words.count > 1 ? words[1] : ""
        n.phoneNumbers = (c.args["phones"] as? [String] ?? []).map { CNLabeledValue(label: CNLabelPhoneNumberMobile, value: CNPhoneNumber(stringValue: $0)) }
        n.emailAddresses = (c.args["emails"] as? [String] ?? []).map { CNLabeledValue(label: CNLabelOther, value: $0 as NSString) }
    }

    private static func write(_ c: PluginCall, _ store: CNContactStore) {
        let save = CNSaveRequest()
        let keys = [CNContactGivenNameKey, CNContactFamilyNameKey, CNContactPhoneNumbersKey, CNContactEmailAddressesKey] as [CNKeyDescriptor]
        do {
            var id = c.string("id") ?? ""
            if c.method == "create" {
                let n = CNMutableContact()
                fill(n, c)
                save.add(n, toContainerWithIdentifier: nil)
                id = n.identifier
            } else {
                guard let found = try? store.unifiedContact(withIdentifier: id, keysToFetch: keys), let n = found.mutableCopy() as? CNMutableContact else {
                    return c.reject(Plugins.notFound, "no such contact")
                }
                if c.method == "update" {
                    fill(n, c)
                    save.update(n)
                } else {
                    save.delete(n)
                }
            }
            try store.execute(save)
            c.resolve(["id": id])
        } catch {
            c.reject(Plugins.failed, error.localizedDescription)
        }
    }

    func contactViewController(_ v: CNContactViewController, didCompleteWith contact: CNContact?) {
        v.dismiss(animated: true)
        call?.resolve(["saved": contact != nil])
        call = nil
    }

    func contactPicker(_ p: CNContactPickerViewController, didSelect contact: CNContact) {
        call?.resolve(Self.dict(contact))
        call = nil
    }

    func contactPickerDidCancel(_ p: CNContactPickerViewController) {
        call?.reject(Plugins.cancelled, "no contact")
        call = nil
    }
}
