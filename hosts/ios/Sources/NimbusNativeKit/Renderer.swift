import UIKit

/// The root of a Nimbus Native app's UI. Make it the window's root view
/// controller; it starts the Go runtime and shows what the app renders.
open class NimbusViewController: UIViewController {
    public let renderer = Renderer()

    open override func loadView() { view = renderer.container }

    open override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
        renderer.start()
    }

    open override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        renderer.sendViewport()
    }

    open override func traitCollectionDidChange(_ previous: UITraitCollection?) {
        super.traitCollectionDidChange(previous)
        renderer.sendViewport()
    }
}

/// Applies the Go runtime's commits to UIKit views and reports taps, edits
/// and the viewport back. One instance per app.
public final class Renderer {
    /// The view the app's root view goes into.
    public let container = UIView()

    /// After each commit is applied: the commit and how long applying took, in
    /// milliseconds. For diagnostics and tests.
    public var onCommit: ((Render.Commit, Double) -> Void)?

    /// Called after the renderer sends a press, before Go has answered.
    public var onPressSent: ((UInt32) -> Void)?

    private var views: [UInt32: UIView] = [:]
    private lazy var scrollReporter = ScrollReporter(renderer: self)
    private var rootTag: UInt32 = 0
    private var lastViewport: Render.Viewport?
    private var started = false

    public init() {}

    public func start() {
        let bridge = Bridge.shared
        bridge.onReceive = { [weak self] data in
            guard let self, let m = try? Render.Message.decode([UInt8](data)) else { return }
            if case .commit(let c) = m { self.apply(c) }
        }
        bridge.onHostCall = { try Renderer.answer($0) }
        do {
            try bridge.start()
            started = true
        } catch {
            let label = UILabel(frame: container.bounds.insetBy(dx: 20, dy: 80))
            label.numberOfLines = 0
            label.text = "Nimbus Native could not start: \(error)"
            container.addSubview(label)
        }
    }

    /// Sends the container's size and safe area if they changed.
    public func sendViewport() {
        guard started else { return }
        let size = container.bounds.size, inset = container.safeAreaInsets
        let v = Render.Viewport(w: Float(size.width), h: Float(size.height), scale: Float(container.traitCollection.displayScale),
                                safeTop: Float(inset.top), safeBottom: Float(inset.bottom),
                                safeLeft: Float(inset.left), safeRight: Float(inset.right),
                                platform: "ios", dark: container.traitCollection.userInterfaceStyle == .dark)
        if v != lastViewport, size.width > 0 {
            lastViewport = v
            send(.viewport(v))
        }
    }

    /// Sends a press as if the user tapped the view (for tests and demos).
    public func press(tag: UInt32) {
        send(.press(Render.Press(tag: tag)))
        onPressSent?(tag)
    }

    /// The tag of the view with this test ID, if it is on screen.
    public func tag(forTestID id: String) -> UInt32? {
        views.first { $0.value.accessibilityIdentifier == id }?.key
    }

    public func view(forTag tag: UInt32) -> UIView? { views[tag] }

    // MARK: applying commits

    private func apply(_ c: Render.Commit) {
        let t0 = CACurrentMediaTime()
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        for op in c.ops {
            guard let op else { continue }
            switch op {
            case .create(let o): create(o)
            case .destroy(let o):
                for t in o.tags { views.removeValue(forKey: t)?.removeFromSuperview() }
            case .children(let o): children(o)
            case .frame(let o):
                views[o.tag]?.frame = CGRect(x: CGFloat(o.x), y: CGFloat(o.y), width: CGFloat(o.w), height: CGFloat(o.h))
            case .box(let o): box(o)
            case .text(let o): (views[o.tag] as? UILabel).map { text($0, o) }
            case .image(let o): (views[o.tag] as? NNImageView)?.load(o)
            case .input(let o): (views[o.tag] as? NNTextField)?.apply(o)
            case .scroll(let o):
                if let s = views[o.tag] as? UIScrollView {
                    s.contentSize = CGSize(width: CGFloat(o.w), height: CGFloat(o.h))
                    s.alwaysBounceVertical = !o.horizontal
                    s.alwaysBounceHorizontal = o.horizontal
                    s.delegate = o.report ? scrollReporter : nil
                }
            case .spinner(let o): (views[o.tag] as? UIActivityIndicatorView)?.color = color(o.color) ?? .gray
            }
        }
        if c.root != 0, let v = views[c.root] {
            views[rootTag]?.removeFromSuperview()
            rootTag = c.root
            container.addSubview(v)
        }
        CATransaction.commit()
        onCommit?(c, (CACurrentMediaTime() - t0) * 1000)
    }

    private func create(_ o: Render.Create) {
        let v: UIView
        switch o.kind {
        case .text:
            let l = UILabel()
            l.numberOfLines = 0
            l.lineBreakMode = .byTruncatingTail
            v = l
        case .image: v = NNImageView()
        case .pressable:
            let p = NNPressable()
            p.addAction(UIAction { [weak self] _ in self?.press(tag: o.tag) }, for: .touchUpInside)
            v = p
        case .scroll: v = UIScrollView()
        case .input:
            let f = NNTextField()
            f.addAction(UIAction { [weak self, weak f] _ in
                self?.send(.changed(Render.Changed(tag: o.tag, text: f?.text ?? "")))
            }, for: .editingChanged)
            v = f
        case .spinner:
            let s = UIActivityIndicatorView(style: .medium)
            s.startAnimating()
            v = s
        default: v = UIView()
        }
        v.tag = Int(o.tag)
        views[o.tag] = v
    }

    /// Makes the managed subviews of a view exactly `children`, in order.
    private func children(_ o: Render.Children) {
        guard let parent = views[o.tag] else { return }
        let wanted = o.children.compactMap { views[$0] }
        let keep = Set(wanted.map(ObjectIdentifier.init))
        for sub in parent.subviews where views[UInt32(truncatingIfNeeded: sub.tag)] === sub && !keep.contains(ObjectIdentifier(sub)) {
            sub.removeFromSuperview()
        }
        for (i, v) in wanted.enumerated() where i >= parent.subviews.count || parent.subviews[i] !== v {
            parent.insertSubview(v, at: i)
        }
    }

    private func box(_ o: Render.Box) {
        guard let v = views[o.tag] else { return }
        v.backgroundColor = color(o.background) ?? .clear // nil paints some views (UILabel) black
        v.layer.cornerRadius = CGFloat(o.radius)
        v.layer.borderWidth = CGFloat(o.borderWidth)
        v.layer.borderColor = color(o.borderColor)?.cgColor
        if let p = v as? NNPressable { p.baseAlpha = CGFloat(o.opacity) } else { v.alpha = CGFloat(o.opacity) }
        if !(v is UIScrollView) { v.clipsToBounds = o.clip }
        v.accessibilityLabel = o.label.isEmpty ? nil : o.label
        v.accessibilityIdentifier = o.testID.isEmpty ? nil : o.testID
        if !o.label.isEmpty { v.isAccessibilityElement = true }
    }

    private func text(_ l: UILabel, _ o: Render.Text) {
        l.text = o.text
        l.font = Renderer.font(o.font)
        l.textColor = color(o.color) ?? .label
        l.textAlignment = o.align == .center ? .center : o.align == .end ? .right : .left
        l.numberOfLines = Int(o.lines)
    }

    fileprivate func send(_ m: Render.Message) { try? Bridge.shared.send(Data(m.encode())) }

    // MARK: answering Go (on a Go thread)

    /// Measures text the way UILabel draws it.
    static func answer(_ data: Data) throws -> Data {
        guard case .measureText(let m) = try Render.Message.decode([UInt8](data)) else { throw WireError.variant }
        let font = font(m.font)
        let maxW = m.maxWidth < 0 ? CGFloat.greatestFiniteMagnitude : CGFloat(m.maxWidth)
        let r = (m.text as NSString).boundingRect(with: CGSize(width: maxW, height: .greatestFiniteMagnitude),
                                                  options: [.usesLineFragmentOrigin, .usesFontLeading],
                                                  attributes: [.font: font], context: nil)
        var h = ceil(r.height)
        if m.lines > 0 { h = min(h, ceil(font.lineHeight * CGFloat(m.lines))) }
        if m.text.isEmpty { h = ceil(font.lineHeight) }
        return Data(Render.Message.textSize(Render.TextSize(w: Float(ceil(r.width)), h: Float(h))).encode())
    }

    static func font(_ f: Render.Font) -> UIFont {
        let weights: [UIFont.Weight] = [.ultraLight, .thin, .light, .regular, .medium, .semibold, .bold, .heavy, .black]
        let w = f.weight == 0 ? 400 : Int(f.weight)
        return .systemFont(ofSize: CGFloat(f.size), weight: weights[max(0, min(8, w / 100 - 1))])
    }
}

/// 0xRRGGBBAA → UIColor; 0 means unset.
func color(_ c: UInt32) -> UIColor? {
    guard c != 0 else { return nil }
    return UIColor(red: CGFloat(c >> 24 & 0xff) / 255, green: CGFloat(c >> 16 & 0xff) / 255,
                   blue: CGFloat(c >> 8 & 0xff) / 255, alpha: CGFloat(c & 0xff) / 255)
}

/// A container that dims while pressed.
final class NNPressable: UIControl {
    var baseAlpha: CGFloat = 1 { didSet { alpha = isHighlighted ? baseAlpha * 0.6 : baseAlpha } }
    override var isHighlighted: Bool { didSet { alpha = isHighlighted ? baseAlpha * 0.6 : baseAlpha } }
}

/// A text field with room around the text (the runtime measures inputs with
/// 10 pt of padding).
final class NNTextField: UITextField {
    private let pad = UIEdgeInsets(top: 10, left: 10, bottom: 10, right: 10)
    override func textRect(forBounds b: CGRect) -> CGRect { b.inset(by: pad) }
    override func editingRect(forBounds b: CGRect) -> CGRect { b.inset(by: pad) }
    override func placeholderRect(forBounds b: CGRect) -> CGRect { b.inset(by: pad) }

    func apply(_ o: Render.Input) {
        if text != o.value { text = o.value }
        placeholder = o.placeholder.isEmpty ? nil : o.placeholder
        font = Renderer.font(o.font)
        textColor = color(o.color) ?? .label
        isSecureTextEntry = o.secure
        switch o.keyboard {
        case .number: keyboardType = .decimalPad
        case .email: keyboardType = .emailAddress
        case .url: keyboardType = .URL
        default: keyboardType = .default
        }
    }
}

/// An image view that loads its picture from a URL, with a shared cache.
final class NNImageView: UIImageView {
    private static let cache = NSCache<NSString, UIImage>()
    private var url = ""

    func load(_ o: Render.Image) {
        switch o.fit {
        case .contain: contentMode = .scaleAspectFit
        case .stretch: contentMode = .scaleToFill
        default: contentMode = .scaleAspectFill
        }
        clipsToBounds = true
        guard o.url != url else { return }
        url = o.url
        image = nil
        if let hit = NNImageView.cache.object(forKey: o.url as NSString) {
            image = hit
            return
        }
        guard let u = URL(string: o.url) else { return }
        URLSession.shared.dataTask(with: u) { [weak self] data, _, _ in
            guard let data, let img = UIImage(data: data) else { return }
            NNImageView.cache.setObject(img, forKey: o.url as NSString)
            DispatchQueue.main.async {
                if self?.url == o.url { self?.image = img }
            }
        }.resume()
    }
}

/// Reports scrolling for scroll views that asked for it (Lists).
final class ScrollReporter: NSObject, UIScrollViewDelegate {
    private weak var renderer: Renderer?

    init(renderer: Renderer) { self.renderer = renderer }

    func scrollViewDidScroll(_ s: UIScrollView) {
        let o = s.contentOffset, b = s.bounds.size
        renderer?.send(.scrolled(Render.Scrolled(tag: UInt32(truncatingIfNeeded: s.tag), x: Float(o.x), y: Float(o.y),
                                                 w: Float(b.width), h: Float(b.height))))
    }
}
