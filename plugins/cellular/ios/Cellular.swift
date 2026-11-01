import CoreTelephony

/// cellular: Core Telephony, which iOS 16 mostly emptied.
enum CellularPlugin {
    private static func generation(_ tech: String?) -> String {
        guard let t = tech else { return "" }
        if #available(iOS 14.1, *), t == CTRadioAccessTechnologyNRNSA || t == CTRadioAccessTechnologyNR { return "5g" }
        switch t {
        case CTRadioAccessTechnologyLTE: return "4g"
        case CTRadioAccessTechnologyGPRS, CTRadioAccessTechnologyEdge, CTRadioAccessTechnologyCDMA1x: return "2g"
        default: return "3g"
        }
    }

    static func handle(_ call: PluginCall) {
        guard call.method == "get" else { return Plugins.unknownMethod(call) }
        let n = CTTelephonyNetworkInfo()
        let c = n.serviceSubscriberCellularProviders?.values.first
        // Since iOS 16 these are placeholders such as "--" and "65535".
        let real = { (s: String?) -> String in
            guard let s, s != "--", s != "65535" else { return "" }
            return s
        }
        call.resolve([
            "hasSim": c != nil, "carrier": real(c?.carrierName), "country": real(c?.isoCountryCode),
            "mcc": real(c?.mobileCountryCode), "mnc": real(c?.mobileNetworkCode),
            "generation": generation(n.serviceCurrentRadioAccessTechnology?.values.first), "roaming": false,
        ])
    }
}
