import Contacts
import ContactsUI
import UIKit

/// contacts: the system picker, and the store with permission.
final class ContactsPlugin: NSObject, CNContactPickerDelegate {
    private static let shared = ContactsPlugin()
    private var call: PluginCall?

    static func handle(_ c: PluginCall) { shared.handle(c) }

    private static func dict(_ c: CNContact) -> [String: Any] {
        ["name": CNContactFormatter.string(from: c, style: .fullName) ?? "",
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
            guard CNContactStore.authorizationStatus(for: .contacts) == .authorized else { return c.reject(Plugins.denied, "contacts are not allowed") }
            DispatchQueue.global(qos: .userInitiated).async {
                let keys: [CNKeyDescriptor] = [CNContactFormatter.descriptorForRequiredKeys(for: .fullName),
                                               CNContactPhoneNumbersKey as CNKeyDescriptor, CNContactEmailAddressesKey as CNKeyDescriptor]
                var out: [[String: Any]] = []
                try? store.enumerateContacts(with: CNContactFetchRequest(keysToFetch: keys)) { x, _ in out.append(Self.dict(x)) }
                c.resolve(["contacts": out])
            }
        default:
            Plugins.unknownMethod(c)
        }
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
