import Foundation

/// localization: the current locale.
enum LocalizationPlugin {
    static func handle(_ call: PluginCall) {
        guard call.method == "get" else { return Plugins.unknownMethod(call) }
        let l = Locale.autoupdatingCurrent
        let languages = Locale.preferredLanguages
        let language = languages.first.map { Locale(identifier: $0).languageCode ?? $0 } ?? ""
        let hours = DateFormatter.dateFormat(fromTemplate: "j", options: 0, locale: l) ?? ""
        call.resolve([
            "languages": languages, "language": language, "region": l.regionCode ?? "",
            "currency": l.currencyCode ?? "", "currencySymbol": l.currencySymbol ?? "",
            "timeZone": TimeZone.current.identifier, "uses24Hour": !hours.contains("a"),
            "firstWeekday": Calendar.current.firstWeekday, "metric": l.usesMetricSystem,
            "decimalSeparator": l.decimalSeparator ?? ".", "groupingSeparator": l.groupingSeparator ?? ",",
            "rightToLeft": Locale.characterDirection(forLanguage: language) == .rightToLeft,
        ])
    }
}
