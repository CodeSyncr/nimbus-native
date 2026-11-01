import StoreKit

/// purchases: StoreKit 2. Signed transactions go to Go as they are; checking
/// them is the server's job.
enum PurchasesPlugin {
    static func handle(_ call: PluginCall) {
        Task {
            do {
                switch call.method {
                case "products":
                    let ids = call.args["ids"] as? [String] ?? []
                    call.resolve(try await Product.products(for: ids).map(describe))
                case "buy":
                    try await buy(call)
                case "finish", "consume":
                    for await v in Transaction.unfinished where String(v.unsafePayloadValue.id) == call.string("id") {
                        await v.unsafePayloadValue.finish()
                    }
                    call.resolve()
                case "owned":
                    call.resolve(await owned())
                case "restore":
                    try await AppStore.sync()
                    call.resolve(await owned())
                default:
                    Plugins.unknownMethod(call)
                }
            } catch StoreKitError.userCancelled {
                call.reject(Plugins.cancelled, "they backed out")
            } catch {
                call.reject(Plugins.failed, error.localizedDescription)
            }
        }
    }

    private static func buy(_ call: PluginCall) async throws {
        guard let p = try await Product.products(for: [call.string("id") ?? ""]).first else {
            return call.reject(Plugins.notFound, "the App Store has no such product")
        }
        switch try await p.purchase() {
        case .success(let v): call.resolve(await receipt(v, finished: false))
        case .userCancelled: call.reject(Plugins.cancelled, "they backed out")
        case .pending: call.resolve(["pending": true])
        @unknown default: call.reject(Plugins.failed, "the App Store gave an unknown answer")
        }
    }

    private static func owned() async -> [[String: Any]] {
        var waiting = Set<UInt64>()
        var out: [[String: Any]] = []
        for await v in Transaction.unfinished {
            waiting.insert(v.unsafePayloadValue.id)
            out.append(await receipt(v, finished: false))
        }
        for await v in Transaction.currentEntitlements where !waiting.contains(v.unsafePayloadValue.id) {
            out.append(await receipt(v, finished: true))
        }
        return out
    }

    private static func receipt(_ v: VerificationResult<Transaction>, finished: Bool) async -> [String: Any] {
        let t = v.unsafePayloadValue
        var r: [String: Any] = ["platform": "apple", "productId": t.productID, "id": String(t.id),
                                "token": v.jwsRepresentation, "finished": finished]
        if let group = t.subscriptionGroupID,
           let list = try? await Product.SubscriptionInfo.status(for: group),
           let s = list.first(where: { (try? $0.transaction.payloadValue.originalID) == t.originalID }) {
            r["renewalInfo"] = s.renewalInfo.jwsRepresentation
        }
        return r
    }

    private static func describe(_ p: Product) -> [String: Any] {
        var r: [String: Any] = ["id": p.id, "title": p.displayName, "description": p.description,
                                "price": p.displayPrice, "amount": NSDecimalNumber(decimal: p.price).doubleValue,
                                "currency": p.priceFormatStyle.currencyCode]
        if let s = p.subscription?.subscriptionPeriod {
            let unit: String
            switch s.unit {
            case .day: unit = "D"
            case .week: unit = "W"
            case .month: unit = "M"
            case .year: unit = "Y"
            @unknown default: unit = "D"
            }
            r["period"] = "P\(s.value)\(unit)"
        }
        return r
    }
}
