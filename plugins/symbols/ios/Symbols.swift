import UIKit

/// symbols: an SF Symbol in an image view (ui.Native "symbol").
enum SymbolsPlugin {
    static func boot() {
        NativeViews.register("symbol") { _ in SymbolHost() }
    }

    static func handle(_ call: PluginCall) { Plugins.unknownMethod(call) }
}

private final class SymbolHost: NimbusNativeView {
    private let image = UIImageView()
    var view: UIView { image }

    init() {
        image.contentMode = .center
        image.isAccessibilityElement = false // the element around it carries the label
    }

    private static let weights: [UIImage.SymbolWeight] = [.ultraLight, .thin, .light, .regular, .medium, .semibold, .bold, .heavy, .black]

    func update(_ p: [String: Any]) {
        let size = CGFloat((p["size"] as? NSNumber)?.floatValue ?? 24)
        let w = (p["weight"] as? NSNumber)?.intValue ?? 400
        let weight = Self.weights[max(0, min(8, (w + 50) / 100 - 1))]
        let c = (p["color"] as? NSNumber)?.intValue ?? 0
        image.tintColor = UIColor(red: CGFloat((c >> 24) & 255) / 255, green: CGFloat((c >> 16) & 255) / 255,
                                  blue: CGFloat((c >> 8) & 255) / 255, alpha: CGFloat(c & 255) / 255)
        image.image = UIImage(systemName: p["name"] as? String ?? "",
                              withConfiguration: UIImage.SymbolConfiguration(pointSize: size, weight: weight))
    }
}
