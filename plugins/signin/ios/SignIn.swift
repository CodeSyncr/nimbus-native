import AuthenticationServices
import CryptoKit
import UIKit

/// signin: Sign in with Apple, and Google's OAuth page in a secure browser
/// sheet, with PKCE so no secret ships in the app.
enum SignInPlugin {
    private static var running: AnyObject?

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "apple": apple(call)
        case "google": google(call)
        default: Plugins.unknownMethod(call)
        }
    }

    private static func window() -> ASPresentationAnchor {
        NimbusViewController.current?.view.window ?? ASPresentationAnchor()
    }

    private static func apple(_ call: PluginCall) {
        let r = ASAuthorizationAppleIDProvider().createRequest()
        r.requestedScopes = [.fullName, .email]
        r.nonce = call.string("nonce")
        let c = ASAuthorizationController(authorizationRequests: [r])
        let f = AppleFlow(call) { running = nil }
        c.delegate = f
        c.presentationContextProvider = f
        running = f
        c.performRequests()
    }

    private final class AppleFlow: NSObject, ASAuthorizationControllerDelegate, ASAuthorizationControllerPresentationContextProviding {
        let call: PluginCall
        let done: () -> Void
        init(_ call: PluginCall, done: @escaping () -> Void) { self.call = call; self.done = done }

        func presentationAnchor(for c: ASAuthorizationController) -> ASPresentationAnchor { SignInPlugin.window() }

        func authorizationController(controller: ASAuthorizationController, didCompleteWithAuthorization a: ASAuthorization) {
            defer { done() }
            guard let c = a.credential as? ASAuthorizationAppleIDCredential,
                  let t = c.identityToken.flatMap({ String(data: $0, encoding: .utf8) }) else {
                return call.reject(Plugins.failed, "Apple sent no token")
            }
            let name = c.fullName.map { PersonNameComponentsFormatter().string(from: $0) } ?? ""
            call.resolve(["idToken": t, "name": name,
                          "code": c.authorizationCode.flatMap { String(data: $0, encoding: .utf8) } ?? ""])
        }

        func authorizationController(controller: ASAuthorizationController, didCompleteWithError e: Error) {
            defer { done() }
            if (e as? ASAuthorizationError)?.code == .canceled { return call.reject(Plugins.cancelled, "they backed out") }
            call.reject(Plugins.failed, e.localizedDescription)
        }
    }

    private final class Anchor: NSObject, ASWebAuthenticationPresentationContextProviding {
        func presentationAnchor(for s: ASWebAuthenticationSession) -> ASPresentationAnchor { SignInPlugin.window() }
    }

    private static func random() -> String {
        var b = [UInt8](repeating: 0, count: 32)
        _ = SecRandomCopyBytes(kSecRandomDefault, b.count, &b)
        return base64URL(Data(b))
    }

    private static func base64URL(_ d: Data) -> String {
        d.base64EncodedString().replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_").replacingOccurrences(of: "=", with: "")
    }

    private static func google(_ call: PluginCall) {
        guard let client = call.string("iosClient"), !client.isEmpty else {
            return call.reject(Plugins.failed, "Google sign-in needs IOSClient")
        }
        let scheme = client.components(separatedBy: ".").reversed().joined(separator: ".")
        let redirect = scheme + ":/oauth2redirect"
        let verifier = random(), state = random()
        var u = URLComponents(string: "https://accounts.google.com/o/oauth2/v2/auth")!
        u.queryItems = [
            .init(name: "client_id", value: client), .init(name: "redirect_uri", value: redirect),
            .init(name: "response_type", value: "code"), .init(name: "scope", value: "openid email profile"),
            .init(name: "state", value: state), .init(name: "code_challenge_method", value: "S256"),
            .init(name: "code_challenge", value: base64URL(Data(SHA256.hash(data: Data(verifier.utf8))))),
        ]
        if let n = call.string("nonce") { u.queryItems?.append(.init(name: "nonce", value: n)) }
        let anchor = Anchor()
        let s = ASWebAuthenticationSession(url: u.url!, callbackURLScheme: scheme) { url, e in
            running = nil
            if (e as? ASWebAuthenticationSessionError)?.code == .canceledLogin {
                return call.reject(Plugins.cancelled, "they backed out")
            }
            let q = url.flatMap { URLComponents(url: $0, resolvingAgainstBaseURL: false)?.queryItems } ?? []
            let value = { (k: String) in q.first { $0.name == k }?.value }
            guard let code = value("code"), value("state") == state else {
                return call.reject(Plugins.failed, e?.localizedDescription ?? value("error") ?? "Google sent no code")
            }
            exchange(call, code, client, redirect, verifier)
        }
        s.presentationContextProvider = anchor
        running = [anchor, s] as NSArray
        s.start()
    }

    private static func exchange(_ call: PluginCall, _ code: String, _ client: String, _ redirect: String, _ verifier: String) {
        var r = URLRequest(url: URL(string: "https://oauth2.googleapis.com/token")!)
        r.httpMethod = "POST"
        r.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        var form = URLComponents()
        form.queryItems = [
            .init(name: "grant_type", value: "authorization_code"), .init(name: "code", value: code),
            .init(name: "client_id", value: client), .init(name: "redirect_uri", value: redirect),
            .init(name: "code_verifier", value: verifier),
        ]
        r.httpBody = form.percentEncodedQuery?.data(using: .utf8)
        URLSession.shared.dataTask(with: r) { data, _, e in
            let answer = data.flatMap { try? JSONSerialization.jsonObject(with: $0) } as? [String: Any]
            guard let t = answer?["id_token"] as? String else {
                return call.reject(Plugins.failed, e?.localizedDescription ?? answer?["error"] as? String ?? "Google sent no token")
            }
            call.resolve(["idToken": t])
        }.resume()
    }
}
