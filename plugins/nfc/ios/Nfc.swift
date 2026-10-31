import CoreNFC

/// nfc: Core NFC's tag reader.
final class NfcPlugin: NSObject, NFCTagReaderSessionDelegate {
    private static let shared = NfcPlugin()
    private var call: PluginCall?
    private var session: NFCTagReaderSession?

    static func handle(_ c: PluginCall) { shared.handle(c) }

    private func handle(_ c: PluginCall) {
        switch c.method {
        case "available":
            c.resolve(["available": NFCTagReaderSession.readingAvailable])
        case "read":
            guard NFCTagReaderSession.readingAvailable else { return c.reject(Plugins.unavailable, "this phone can't read NFC") }
            call = c
            session = NFCTagReaderSession(pollingOption: [.iso14443, .iso15693, .iso18092], delegate: self)
            session?.alertMessage = c.string("prompt") ?? "Hold a tag near the phone"
            session?.begin()
        default:
            Plugins.unknownMethod(c)
        }
    }

    func tagReaderSessionDidBecomeActive(_ s: NFCTagReaderSession) {}

    func tagReaderSession(_ s: NFCTagReaderSession, didInvalidateWithError e: Error) {
        let cancelled = (e as? NFCReaderError)?.code == .readerSessionInvalidationErrorUserCanceled
        call?.reject(cancelled ? Plugins.cancelled : Plugins.failed, e.localizedDescription)
        call = nil
    }

    func tagReaderSession(_ s: NFCTagReaderSession, didDetect tags: [NFCTag]) {
        guard let tag = tags.first else { return }
        s.connect(to: tag) { [self] e in
            if let e { return s.invalidate(errorMessage: e.localizedDescription) }
            var id = Data()
            var ndef: NFCNDEFTag?
            switch tag {
            case .miFare(let t): id = t.identifier; ndef = t
            case .iso15693(let t): id = t.identifier; ndef = t
            case .iso7816(let t): id = t.identifier; ndef = t
            case .feliCa(let t): id = t.currentIDm; ndef = t
            @unknown default: break
            }
            let hex = id.map { String(format: "%02x", $0) }.joined()
            let finish = { (records: [String]) in
                self.call?.resolve(["id": hex, "records": records])
                self.call = nil
                s.invalidate()
            }
            guard let ndef else { return finish([]) }
            ndef.readNDEF { m, _ in
                finish((m?.records ?? []).map { r in
                    r.wellKnownTypeURIPayload()?.absoluteString ?? r.wellKnownTypeTextPayload().0 ?? ""
                })
            }
        }
    }
}
