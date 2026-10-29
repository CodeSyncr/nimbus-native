import UIKit

/// The root of a Nimbus Native app's UI. Make it the window's root view
/// controller; it starts the Go runtime and shows what the app renders.
open class NimbusViewController: UIViewController {
    public let renderer: Renderer

    /// schema is the app's generated NimbusSchema.combined.
    public init(schema: UInt64) {
        renderer = Renderer(schema: schema)
        super.init(nibName: nil, bundle: nil)
    }

    public required init?(coder: NSCoder) { fatalError("create NimbusViewController in code, with init(schema:)") }

    open override func loadView() {
        view = renderer.container
        renderer.viewController = self
    }

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

    /// The view controller hosting the UI; navigation stacks become its children.
    public weak var viewController: UIViewController?

    private var screens: [UInt32: NNScreenController] = [:]
    private var tabControllers: [UInt32: NNTabController] = [:]
    private var modals: [UInt32: NNModalController] = [:]

    /// The theme for what UIKit draws itself (bars, backgrounds); see Chrome.
    private(set) var chrome = Render.Chrome()

    private var views: [UInt32: UIView] = [:]
    private var radii: [UInt32: CGFloat] = [:] // as asked; drawn at most half the short side
    private lazy var scrollReporter = ScrollReporter(renderer: self)
    private var rootTag: UInt32 = 0
    private var lastViewport: Render.Viewport?
    private var started = false
    private let schema: UInt64

    /// schema is the app's generated NimbusSchema.combined.
    public init(schema: UInt64) { self.schema = schema }

    public func start() {
        let bridge = Bridge.shared
        bridge.onReceive = { [weak self] data in
            guard let self, let m = try? Render.Message.decode([UInt8](data)) else { return }
            if case .commit(let c) = m { self.apply(c) }
        }
        bridge.onHostCall = { try Renderer.answer($0) }
        do {
            try bridge.start(schema: schema)
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
                // Detach only the top of each destroyed subtree: its
                // descendants go with it. A stack screen's root stays in its
                // controller, which may be animating out, and goes with it.
                let gone = Set(o.tags.compactMap { views[$0] }.map(ObjectIdentifier.init))
                for t in o.tags {
                    radii.removeValue(forKey: t)
                    let v = views.removeValue(forKey: t)
                    if let m = modals.removeValue(forKey: t) {
                        // Its controller keeps the views while it animates away.
                        if m.presentingViewController != nil { m.dismiss(animated: true) }
                        views.removeValue(forKey: t)
                        continue
                    }
                    let isScreen = screens.removeValue(forKey: t) != nil || tabControllers.removeValue(forKey: t) != nil
                    if let v, !isScreen, !(v.superview.map { gone.contains(ObjectIdentifier($0)) } ?? false) {
                        v.removeFromSuperview()
                    }
                }
            case .children(let o): children(o)
            case .frame(let o):
                guard let v = views[o.tag] else { break }
                v.frame = CGRect(x: CGFloat(o.x), y: CGFloat(o.y), width: CGFloat(o.w), height: CGFloat(o.h))
                if let r = radii[o.tag] { round(v, r) }
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
            case .screen(let o): screen(o.tag).title = o.title
            case .tabItem(let o):
                tab(o.tag).tabBarItem = UITabBarItem(title: o.title, image: o.icon.isEmpty ? nil : UIImage(systemName: o.icon), tag: 0)
            case .tabSelect(let o): (views[o.tag] as? NNTabsView)?.select(Int(o.index))
            case .chrome(let o): applyChrome(o)
            case .present(let o):
                if let m = modals[o.tag], m.presentingViewController == nil, var top = viewController {
                    while let p = top.presentedViewController { top = p }
                    top.present(m, animated: true)
                }
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
            let l = NNLabel()
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
        case .stack:
            let st = NNStackView(renderer: self, tag: o.tag)
            st.style(chrome)
            v = st
        case .tabs:
            let tb = NNTabsView(renderer: self, tag: o.tag)
            tb.style(chrome)
            v = tb
        case .modal:
            v = UIView()
            let m = NNModalController(renderer: self, tag: o.tag, content: v)
            m.loadViewIfNeeded()
            m.view.backgroundColor = color(chrome.background)
            modals[o.tag] = m
        default: v = UIView()
        }
        v.tag = Int(o.tag)
        views[o.tag] = v
    }

    /// Makes the managed subviews of a view exactly `children`, in order.
    private func children(_ o: Render.Children) {
        guard let parent = views[o.tag] else { return }
        let wanted = o.children.compactMap { views[$0] }
        if let stack = parent as? NNStackView {
            // A stack's children are screens: each goes in its own controller.
            stack.setScreens(o.children.compactMap { t in views[t].map { screen(t).holding($0, in: stack) } })
            return
        }
        if let tabs = parent as? NNTabsView {
            tabs.setTabs(o.children.compactMap { t in views[t].map { tab(t).holding($0, in: tabs) } })
            return
        }
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
        radii[o.tag] = o.radius > 0 ? CGFloat(o.radius) : nil
        round(v, CGFloat(o.radius))
        v.layer.borderWidth = CGFloat(o.borderWidth)
        v.layer.borderColor = color(o.borderColor)?.cgColor
        if let p = v as? NNPressable { p.baseAlpha = CGFloat(o.opacity) } else { v.alpha = CGFloat(o.opacity) }
        if !(v is UIScrollView) { v.clipsToBounds = o.clip }
        v.accessibilityLabel = o.label.isEmpty ? nil : o.label
        v.accessibilityIdentifier = o.testID.isEmpty ? nil : o.testID
        if !o.label.isEmpty { v.isAccessibilityElement = true }
    }

    /// Core Animation draws nothing for a corner radius over half the short
    /// side, so a pill (Radius: 999) is clamped to a full round end.
    private func round(_ v: UIView, _ r: CGFloat) {
        v.layer.cornerRadius = min(r, min(v.bounds.width, v.bounds.height) / 2)
    }

    private func text(_ l: UILabel, _ o: Render.Text) {
        l.text = o.text
        l.font = Renderer.font(o.font)
        l.textColor = color(o.color) ?? .label
        l.textAlignment = o.align == .center ? .center : o.align == .end ? .right : .left
        l.numberOfLines = Int(o.lines)
    }

    fileprivate func send(_ m: Render.Message) { try? Bridge.shared.send(Data(m.encode())) }

    private func applyChrome(_ c: Render.Chrome) {
        chrome = c
        container.backgroundColor = color(c.background)
        for v in views.values {
            (v as? NNStackView)?.style(c)
            (v as? NNTabsView)?.style(c)
        }
        let backgrounds: [UIViewController] = Array(screens.values) + Array(tabControllers.values) + Array(modals.values)
        for vc in backgrounds where vc.isViewLoaded { vc.view.backgroundColor = color(c.background) }
    }

    private func tab(_ tag: UInt32) -> NNTabController {
        if let c = tabControllers[tag] { return c }
        let c = NNTabController()
        c.loadViewIfNeeded()
        c.view.backgroundColor = color(chrome.background)
        tabControllers[tag] = c
        return c
    }

    private func screen(_ tag: UInt32) -> NNScreenController {
        if let c = screens[tag] { return c }
        let c = NNScreenController(renderer: self)
        c.loadViewIfNeeded()
        c.view.backgroundColor = color(chrome.background)
        screens[tag] = c
        return c
    }

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

/// A label that draws its text at the top of its frame, as Go's layout and
/// Android do (UILabel centres it vertically when the frame is taller).
final class NNLabel: UILabel {
    override func drawText(in rect: CGRect) {
        let fit = textRect(forBounds: rect, limitedToNumberOfLines: numberOfLines)
        super.drawText(in: CGRect(x: rect.minX, y: rect.minY, width: rect.width, height: min(fit.height, rect.height)))
    }
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

/// A navigation stack: a UINavigationController whose screens' content is
/// Go-rendered views. It animates when Go pushes or pops one screen, and
/// tells Go when the user goes back.
final class NNStackView: UIView, UINavigationControllerDelegate {
    let nav = UINavigationController()
    let stackTag: UInt32
    private weak var renderer: Renderer?
    private var depth = 0 // the depth Go last asked for

    init(renderer: Renderer, tag: UInt32) {
        self.renderer = renderer
        stackTag = tag
        super.init(frame: .zero)
        nav.delegate = self
        nav.view.frame = bounds
        nav.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        addSubview(nav.view)
    }

    required init?(coder: NSCoder) { fatalError("not used") }

    override func didMoveToWindow() {
        super.didMoveToWindow()
        if window != nil, nav.parent == nil, let host = owningViewController(self) ?? renderer?.viewController {
            host.addChild(nav)
            nav.didMove(toParent: host)
        }
    }

    /// Bar colors from the theme.
    func style(_ c: Render.Chrome) {
        guard c.bar != 0 else { return }
        let a = UINavigationBarAppearance()
        a.configureWithOpaqueBackground()
        a.backgroundColor = color(c.bar)
        a.shadowColor = color(c.border)
        if let t = color(c.barText) {
            a.titleTextAttributes = [.foregroundColor: t]
            a.largeTitleTextAttributes = [.foregroundColor: t]
        }
        let bar = nav.navigationBar
        bar.standardAppearance = a
        bar.scrollEdgeAppearance = a
        bar.compactAppearance = a
        bar.tintColor = color(c.accent)
    }

    func setScreens(_ controllers: [UIViewController]) {
        let current = nav.viewControllers
        depth = controllers.count
        let animated = window != nil
        if controllers.count == current.count + 1, Array(controllers.prefix(current.count)) == current {
            nav.pushViewController(controllers.last!, animated: animated)
        } else if controllers.count == current.count - 1, Array(current.prefix(controllers.count)) == controllers {
            nav.popViewController(animated: animated)
        } else if controllers != current {
            nav.setViewControllers(controllers, animated: false)
        }
    }

    func navigationController(_ n: UINavigationController, didShow vc: UIViewController, animated: Bool) {
        // Fewer screens than Go asked for: the user went back.
        if n.viewControllers.count < depth {
            depth = n.viewControllers.count
            renderer?.send(.popped(Render.Popped(tag: stackTag, depth: UInt32(depth))))
        }
    }

    func reportSize(_ size: CGSize) {
        renderer?.send(.resized(Render.Resized(tag: stackTag, w: Float(size.width), h: Float(size.height))))
    }
}

/// One stack screen: its view sits below the navigation bar and holds the
/// screen's Go-rendered root view.
final class NNScreenController: UIViewController {
    private weak var renderer: Renderer?
    private weak var stack: NNStackView?
    private var reported = CGSize.zero

    init(renderer: Renderer) {
        self.renderer = renderer
        super.init(nibName: nil, bundle: nil)
        edgesForExtendedLayout = []
    }

    required init?(coder: NSCoder) { fatalError("not used") }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
    }

    func holding(_ content: UIView, in stack: NNStackView) -> NNScreenController {
        self.stack = stack
        if content.superview !== view { view.addSubview(content) }
        return self
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        let size = view.bounds.size
        if size != reported, size.width > 0 {
            reported = size
            stack?.reportSize(size)
        }
    }
}

/// The view controller whose view contains v (for nesting stacks in tabs).
func owningViewController(_ v: UIView) -> UIViewController? {
    var r: UIResponder? = v.next
    while let x = r {
        if let vc = x as? UIViewController { return vc }
        r = x.next
    }
    return nil
}

/// A tab bar: a UITabBarController whose tabs' content is Go-rendered views.
/// It tells Go which tab the user picks and how much room tabs have.
final class NNTabsView: UIView, UITabBarControllerDelegate {
    let bar = UITabBarController()
    let tabsTag: UInt32
    private weak var renderer: Renderer?

    init(renderer: Renderer, tag: UInt32) {
        self.renderer = renderer
        tabsTag = tag
        super.init(frame: .zero)
        bar.delegate = self
        bar.view.frame = bounds
        bar.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        addSubview(bar.view)
    }

    required init?(coder: NSCoder) { fatalError("not used") }

    override func didMoveToWindow() {
        super.didMoveToWindow()
        if window != nil, bar.parent == nil, let host = owningViewController(self) ?? renderer?.viewController {
            host.addChild(bar)
            bar.didMove(toParent: host)
        }
    }

    /// Tab bar colors from the theme.
    func style(_ c: Render.Chrome) {
        guard c.bar != 0 else { return }
        let a = UITabBarAppearance()
        a.configureWithOpaqueBackground()
        a.backgroundColor = color(c.bar)
        a.shadowColor = color(c.border)
        for item in [a.stackedLayoutAppearance, a.inlineLayoutAppearance, a.compactInlineLayoutAppearance] {
            item.normal.iconColor = color(c.textMuted)
            item.normal.titleTextAttributes = [.foregroundColor: color(c.textMuted) ?? .gray]
            item.selected.iconColor = color(c.accent)
            item.selected.titleTextAttributes = [.foregroundColor: color(c.accent) ?? .systemBlue]
        }
        bar.tabBar.standardAppearance = a
        bar.tabBar.scrollEdgeAppearance = a
        bar.tabBar.tintColor = color(c.accent)
    }

    func setTabs(_ controllers: [UIViewController]) {
        if bar.viewControllers ?? [] != controllers {
            bar.setViewControllers(controllers, animated: false)
        }
    }

    func select(_ i: Int) {
        if i < (bar.viewControllers?.count ?? 0) { bar.selectedIndex = i }
    }

    func tabBarController(_ c: UITabBarController, didSelect vc: UIViewController) {
        renderer?.send(.selected(Render.Selected(tag: tabsTag, index: UInt32(c.selectedIndex))))
    }

    func reportSize(_ size: CGSize, bottom: CGFloat) {
        renderer?.send(.resized(Render.Resized(tag: tabsTag, w: Float(size.width), h: Float(size.height), bottom: Float(bottom))))
    }
}

/// One tab: its Go-rendered root view fills the screen, under the status bar
/// and the translucent tab bar, as UIKit expects; Go is told how much of the
/// bottom the bar covers. A stack inside stops above the bar by itself.
final class NNTabController: UIViewController {
    private weak var tabs: NNTabsView?
    private var reported = CGSize.zero, reportedBottom: CGFloat = -1

    func holding(_ content: UIView, in tabs: NNTabsView) -> NNTabController {
        self.tabs = tabs
        if content.superview !== view { view.addSubview(content) }
        return self
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        let size = view.bounds.size, bottom = view.safeAreaInsets.bottom
        if (size != reported || bottom != reportedBottom), size.width > 0 {
            reported = size
            reportedBottom = bottom
            tabs?.reportSize(size, bottom: bottom)
        }
    }
}

/// A modal: a page sheet holding a Go-rendered tree. Swiping it down tells Go;
/// Go dismissing it animates it away.
final class NNModalController: UIViewController, UIAdaptivePresentationControllerDelegate {
    private weak var renderer: Renderer?
    private let modalTag: UInt32
    private let content: UIView
    private var reported = CGSize.zero

    init(renderer: Renderer, tag: UInt32, content: UIView) {
        self.renderer = renderer
        modalTag = tag
        self.content = content
        super.init(nibName: nil, bundle: nil)
        modalPresentationStyle = .pageSheet
    }

    required init?(coder: NSCoder) { fatalError("not used") }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.addSubview(content)
        presentationController?.delegate = self
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        let size = view.bounds.size
        if size != reported, size.width > 0 {
            reported = size
            renderer?.send(.resized(Render.Resized(tag: modalTag, w: Float(size.width), h: Float(size.height))))
        }
    }

    func presentationControllerDidDismiss(_ p: UIPresentationController) {
        renderer?.send(.dismissed(Render.Dismissed(tag: modalTag)))
    }
}
