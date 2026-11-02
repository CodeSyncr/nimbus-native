import AuthenticationServices
import UIKit

/// oauth: the provider's page in ASWebAuthenticationSession, which hands back
/// the address it was sent to. Go does the rest: PKCE, state, the exchange.
enum OAuthPlugin {
    private static var running: (ASWebAuthenticationSession, Anchor)?

    private final class Anchor: NSObject, ASWebAuthenticationPresentationContextProviding {
        func presentationAnchor(for s: ASWebAuthenticationSession) -> ASPresentationAnchor {
            NimbusViewController.current?.view.window ?? ASPresentationAnchor()
        }
    }

    static func handle(_ call: PluginCall) {
        guard call.method == "open" else { return Plugins.unknownMethod(call) }
        guard let s = call.string("url"), let page = URL(string: s), page.scheme == "https" || page.scheme == "http" else {
            return call.reject(Plugins.failed, "that isn't a web address")
        }
        guard let scheme = call.string("scheme"), !scheme.isEmpty else { return call.reject(Plugins.failed, "no redirect scheme") }
        running?.0.cancel() // one sign-in at a time
        let anchor = Anchor()
        var session: ASWebAuthenticationSession?
        session = ASWebAuthenticationSession(url: page, callbackURLScheme: scheme) { url, e in
            let mine = session
            session = nil // the session held itself through this closure
            if running?.0 === mine { running = nil } // a cancelled earlier one mustn't clear this one
            if let url { return call.resolve(["url": url.absoluteString]) }
            if (e as? ASWebAuthenticationSessionError)?.code == .canceledLogin {
                return call.reject(Plugins.cancelled, "they backed out")
            }
            call.reject(Plugins.failed, e?.localizedDescription ?? "the sign-in sheet closed")
        }
        guard let session else { return call.reject(Plugins.failed, "the sign-in sheet didn't open") }
        session.presentationContextProvider = anchor
        session.prefersEphemeralWebBrowserSession = call.args["ephemeral"] as? Bool ?? false
        running = (session, anchor)
        if !session.start() {
            running = nil
            call.reject(Plugins.failed, "the sign-in sheet didn't open")
        }
    }
}
