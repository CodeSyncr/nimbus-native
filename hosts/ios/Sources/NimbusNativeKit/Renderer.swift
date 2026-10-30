import CoreText
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
        // The keyboard covers part of the screen: tell Go how much, so SafeArea pads clear of it.
        let center = NotificationCenter.default
        center.addObserver(self, selector: #selector(keyboardChanged(_:)), name: UIResponder.keyboardWillChangeFrameNotification, object: nil)
        center.addObserver(self, selector: #selector(keyboardChanged(_:)), name: UIResponder.keyboardWillHideNotification, object: nil)
    }

    @objc private func keyboardChanged(_ n: Notification) {
        var overlap: CGFloat = 0
        if n.name != UIResponder.keyboardWillHideNotification,
           let end = (n.userInfo?[UIResponder.keyboardFrameEndUserInfoKey] as? NSValue)?.cgRectValue {
            let kb = view.convert(end, from: nil)
            overlap = max(0, view.bounds.maxY - kb.minY)
        }
        renderer.keyboard = overlap
        renderer.sendViewport()
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
    private var decor: [UInt32: Decor] = [:] // blur, gradient and shadow, laid out with the view
    private var hidden: Set<UInt32> = [] // tags hidden by Style.Hidden
    private var transitions: [UInt32: UInt32] = [:] // ms, for views whose changes animate
    private lazy var scrollReporter = ScrollReporter(renderer: self)
    private var rootTag: UInt32 = 0
    private var lastViewport: Render.Viewport?
    /// How far the keyboard covers the container's bottom (NimbusViewController keeps it current).
    public var keyboard: CGFloat = 0
    private var started = false
    private let schema: UInt64

    /// schema is the app's generated NimbusSchema.combined.
    public init(schema: UInt64) {
        self.schema = schema
        // Where a view is in the window, for what opens beside it (ui.Menu).
        Plugins.register("nimbusView") { [weak self] call in
            guard let tag = (call.args["tag"] as? NSNumber)?.uint32Value, let v = self?.views[tag] else {
                return call.reject(Plugins.notFound, "no such view")
            }
            let f = v.convert(v.bounds, to: nil)
            call.resolve(["X": f.minX, "Y": f.minY, "W": f.width, "H": f.height])
        }
    }

    public func start() {
        let bridge = Bridge.shared
        bridge.onReceive = { [weak self] data in
            guard let self, let m = try? Render.Message.decode([UInt8](data)) else { return }
            if case .commit(let c) = m { self.apply(c) }
        }
        bridge.onHostCall = { try Renderer.answer($0) }
        do {
            let fresh = try bridge.start(schema: schema)
            started = true
            // A runtime already running drew into views that are gone: draw it all again here.
            if !fresh { send(.reattach(Render.Reattach(reason: "view controller created again"))) }
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
                                platform: "ios", dark: container.traitCollection.userInterfaceStyle == .dark,
                                keyboard: Float(keyboard))
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

    /// Goes back one step, as the system gesture would: the top modal's own
    /// stack, else the modal itself, else the visible stack. Returns false
    /// when there is nowhere to go. (Android's back button calls the same
    /// method in its kit; on iOS, tests and deep links use it.)
    @discardableResult
    public func back() -> Bool {
        var top = viewController
        while let presented = top?.presentedViewController { top = presented }
        let stacks = views.values.compactMap { $0 as? NNStackView }.filter { $0.window != nil && $0.nav.viewControllers.count > 1 }
        if let modal = top as? NNModalController {
            if let s = stacks.first(where: { $0.isDescendant(of: modal.view) }) {
                s.nav.popViewController(animated: true) // reported to Go as Popped
                return true
            }
            modal.close()
            return true
        }
        guard let s = stacks.first else { return false }
        s.nav.popViewController(animated: true)
        return true
    }

    /// Opens a link to the app (myapp://orders/42): its page shows, pushed on
    /// the navigation stack on screen, or as a sheet.
    public func open(_ url: URL) {
        send(.open(Render.Open(url: url.absoluteString)))
    }

    /// Types into an input as the user would, replacing its text (for tests and demos).
    public func type(tag: UInt32, text: String) {
        (views[tag] as? NNInput)?.userTyped(text)
    }

    /// Pull to refresh on a scroll view: a UIRefreshControl that reports pulls.
    private func refresh(_ s: UIScrollView, tag: UInt32, _ o: Render.Scroll) {
        guard o.refreshable else {
            s.refreshControl = nil
            return
        }
        let rc = s.refreshControl ?? {
            let rc = UIRefreshControl()
            rc.addAction(UIAction { [weak self] _ in self?.send(.refresh(Render.Refresh(tag: tag))) }, for: .valueChanged)
            s.refreshControl = rc
            return rc
        }()
        if o.refreshing, !rc.isRefreshing {
            rc.beginRefreshing()
        } else if !o.refreshing, rc.isRefreshing {
            rc.endRefreshing()
        }
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
                    transitions.removeValue(forKey: t)
                    decor.removeValue(forKey: t)
                    hidden.remove(t)
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
                let f = CGRect(x: CGFloat(o.x), y: CGFloat(o.y), width: CGFloat(o.w), height: CGFloat(o.h))
                // A transformed view (Style.Scale, Rotate) is placed by bounds and centre:
                // its frame is undefined under a transform.
                let place = {
                    if v.transform.isIdentity { v.frame = f } else {
                        v.bounds.size = f.size
                        v.center = CGPoint(x: f.midX, y: f.midY)
                    }
                }
                if let ms = transitions[o.tag], v.bounds.size != .zero {
                    animate(ms) {
                        place()
                        if let r = self.radii[o.tag] { self.round(v, r) }
                        self.decor[o.tag]?.layout(v)
                    }
                } else {
                    place()
                    if let r = radii[o.tag] { round(v, r) }
                    decor[o.tag]?.layout(v)
                }
            case .box(let o): box(o)
            case .draw(let o): (views[o.tag] as? NNCanvas)?.set(o)
            case .text(let o): (views[o.tag] as? UILabel).map { text($0, o) }
            case .image(let o): (views[o.tag] as? NNImageView)?.load(o)
            case .input(let o): (views[o.tag] as? NNInput)?.apply(o)
            case .scroll(let o):
                if let s = views[o.tag] as? UIScrollView {
                    let grew = CGFloat(o.h) > s.contentSize.height
                    s.contentSize = CGSize(width: CGFloat(o.w), height: CGFloat(o.h))
                    if o.stickToEnd, grew, s.window != nil {
                        // A chat: show the newest at the end.
                        let end = max(-s.adjustedContentInset.top, s.contentSize.height - s.bounds.height + s.adjustedContentInset.bottom)
                        s.setContentOffset(CGPoint(x: s.contentOffset.x, y: end), animated: true)
                    }
                    s.alwaysBounceVertical = !o.horizontal
                    s.alwaysBounceHorizontal = o.horizontal
                    s.delegate = o.report ? scrollReporter : nil
                    refresh(s, tag: o.tag, o)
                }
            case .spinner(let o): (views[o.tag] as? UIActivityIndicatorView)?.color = color(o.color) ?? .gray
            case .slider(let o):
                if let sl = views[o.tag] as? UISlider {
                    sl.minimumValue = o.min
                    sl.maximumValue = o.max
                    if !sl.isTracking, sl.value != o.value { sl.setValue(o.value, animated: sl.window != nil) }
                    sl.isEnabled = o.enabled
                    sl.minimumTrackTintColor = color(o.tint)
                }
            case .switch(let o):
                if let sw = views[o.tag] as? UISwitch {
                    if sw.isOn != o.on { sw.setOn(o.on, animated: sw.window != nil) }
                    sw.isEnabled = o.enabled
                    sw.onTintColor = color(o.tint)
                }
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
            let input = NNInput { [weak self] text in self?.send(.changed(Render.Changed(tag: o.tag, text: text))) }
            input.onSubmit = { [weak self] in self?.send(.submitted(Render.Submitted(tag: o.tag))) }
            v = input
        case .slider:
            let sl = UISlider()
            sl.addAction(UIAction { [weak self, weak sl] _ in
                guard let sl else { return }
                self?.send(.slid(Render.Slid(tag: o.tag, value: sl.value)))
            }, for: .valueChanged)
            v = sl
        case .switch:
            let sw = UISwitch()
            sw.addAction(UIAction { [weak self, weak sw] _ in
                guard let sw else { return }
                self?.send(.toggled(Render.Toggled(tag: o.tag, on: sw.isOn)))
            }, for: .valueChanged)
            v = sw
        case .canvas: v = NNCanvas()
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
        // A blur stays under the children (Decor).
        let base = parent.subviews.first is NNBackdrop ? 1 : 0
        for (i, v) in wanted.enumerated() where base + i >= parent.subviews.count || parent.subviews[base + i] !== v {
            parent.insertSubview(v, at: base + i)
        }
        // A child put at the bottom went under the gradient: it stays below them.
        decor[o.tag]?.keepGradientBelow(parent)
    }

    private func box(_ o: Render.Box) {
        guard let v = views[o.tag] else { return }
        let animated = transitions[o.tag] != nil && v.window != nil
        transitions[o.tag] = o.transition > 0 ? o.transition : nil
        let d = decor[o.tag] ?? Decor()
        d.apply(o, to: v)
        decor[o.tag] = d.isEmpty ? nil : d
        let tint = color(o.background) ?? .clear // nil paints some views (UILabel) black
        let scale = CGFloat(o.scale == 0 ? 1 : o.scale)
        let transform = CGAffineTransform(scaleX: scale, y: scale).rotated(by: CGFloat(o.rotate) * .pi / 180)
        let look = {
            if v.transform != transform {
                // Keep the view where layout put it while the transform changes.
                let center = v.center
                v.transform = transform
                v.center = center
            }
            // With a blur the tint goes over it, in the blur's own view.
            if let b = d.blur { b.contentView.backgroundColor = d.glass ? .clear : tint; v.backgroundColor = .clear } else { v.backgroundColor = tint }
            if let p = v as? NNPressable { p.baseAlpha = CGFloat(o.opacity) } else { v.alpha = CGFloat(o.opacity) }
        }
        if animated, o.transition > 0 { animate(o.transition, look) } else { look() }
        radii[o.tag] = o.radius > 0 ? CGFloat(o.radius) : nil
        round(v, CGFloat(o.radius))
        v.layer.maskedCorners = Decor.corners(o.corners)
        d.layout(v)
        v.layer.borderWidth = CGFloat(o.borderWidth)
        v.layer.borderColor = color(o.borderColor)?.cgColor
        if o.longPress, let p = v as? NNPressable {
            let tag = o.tag
            p.reportLongPress { [weak self] in self?.send(.longPressed(Render.LongPressed(tag: tag))) }
        }
        gestures(v, o)
        if !(v is UIScrollView) { v.clipsToBounds = o.clip }
        // Only a change of Style.Hidden touches isHidden.
        if o.hidden { if hidden.insert(o.tag).inserted { v.isHidden = true } }
        else if hidden.remove(o.tag) != nil { v.isHidden = false }
        v.accessibilityLabel = o.label.isEmpty ? nil : o.label
        v.accessibilityIdentifier = o.testID.isEmpty ? nil : o.testID
        if !o.label.isEmpty { v.isAccessibilityElement = true }
    }

    /// Runs changes as an animation of ms milliseconds (Style.Transition).
    private func animate(_ ms: UInt32, _ changes: @escaping () -> Void) {
        UIView.animate(withDuration: Double(ms) / 1000, delay: 0, options: [.curveEaseInOut, .beginFromCurrentState, .allowUserInteraction], animations: changes)
    }

    /// Core Animation draws nothing for a corner radius over half the short
    /// side, so a pill (Radius: 999) is clamped to a full round end.
    private func round(_ v: UIView, _ r: CGFloat) {
        v.layer.cornerRadius = min(r, min(v.bounds.width, v.bounds.height) / 2)
    }

    private func text(_ l: UILabel, _ o: Render.Text) {
        l.font = Renderer.font(o.font)
        if !o.spans.isEmpty {
            l.attributedText = Renderer.rich(o.spans, base: o.font)
        } else if let attrs = Renderer.extraAttributes(o.font) {
            l.attributedText = NSAttributedString(string: o.text, attributes: attrs)
        } else {
            l.attributedText = nil
            l.text = o.text
        }
        if o.spans.isEmpty { l.textColor = color(o.color) ?? .label } // spans carry their own
        l.textAlignment = o.align == .center ? .center : o.align == .end ? .right : .left
        l.numberOfLines = Int(o.lines)
        // Shrink to fit (Style.MinScale): one line, smaller rather than cut.
        l.adjustsFontSizeToFitWidth = o.minScale > 0
        l.minimumScaleFactor = o.minScale > 0 ? CGFloat(o.minScale) : 0
    }

    /// Adds or removes a view's drag and pinch recognizers to match Style.
    private func gestures(_ v: UIView, _ o: Render.Box) {
        let tag = o.tag
        let pan = v.gestureRecognizers?.first { $0 is NNPan } as? NNPan
        if o.pan == 0 {
            if let pan { v.removeGestureRecognizer(pan) }
        } else if let pan {
            pan.axis = o.pan
        } else {
            let g = NNPan { [weak self] phase, t, s in
                self?.send(.panned(Render.Panned(tag: tag, phase: phase, x: Float(t.x), y: Float(t.y), vx: Float(s.x), vy: Float(s.y))))
            }
            g.axis = o.pan
            v.addGestureRecognizer(g)
            v.isUserInteractionEnabled = true
        }
        let pinch = v.gestureRecognizers?.first { $0 is NNPinch }
        if !o.pinch {
            if let pinch { v.removeGestureRecognizer(pinch) }
        } else if pinch == nil {
            v.addGestureRecognizer(NNPinch { [weak self] phase, scale in
                self?.send(.pinched(Render.Pinched(tag: tag, phase: phase, scale: scale)))
            })
            v.isUserInteractionEnabled = true
        }
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

    /// Answers Go's synchronous calls: text measurement, and starting plugin calls.
    static func answer(_ data: Data) throws -> Data {
        switch try Render.Message.decode([UInt8](data)) {
        case .measureText(let m): return measure(m)
        case .pluginCall(let c): return Data(Render.Message.pluginAck(Plugins.start(c)).encode())
        default: throw WireError.variant
        }
    }

    /// Measures text the way UILabel draws it.
    static func measure(_ m: Render.MeasureText) -> Data {
        let font = font(m.font)
        let maxW = m.maxWidth < 0 ? CGFloat.greatestFiniteMagnitude : CGFloat(m.maxWidth)
        var attrs: [NSAttributedString.Key: Any] = [.font: font]
        extraAttributes(m.font)?.forEach { attrs[$0.key] = $0.value }
        let size = CGSize(width: maxW, height: .greatestFiniteMagnitude)
        let r = m.spans.isEmpty
            ? (m.text as NSString).boundingRect(with: size, options: [.usesLineFragmentOrigin, .usesFontLeading], attributes: attrs, context: nil)
            : rich(m.spans, base: m.font).boundingRect(with: size, options: [.usesLineFragmentOrigin, .usesFontLeading], context: nil)
        let line = m.font.lineHeight > 0 ? CGFloat(m.font.lineHeight) : font.lineHeight
        var h = ceil(r.height)
        if m.lines > 0 { h = min(h, ceil(line * CGFloat(m.lines))) }
        if m.text.isEmpty { h = ceil(line) }
        return Data(Render.Message.textSize(Render.TextSize(w: Float(ceil(r.width)), h: Float(h))).encode())
    }

    /// A rich text (ui.Rich): each run in its own font, spacing and color, the
    /// paragraph's line height from the text's own font.
    static func rich(_ spans: [Render.Span], base: Render.Font) -> NSAttributedString {
        let out = NSMutableAttributedString()
        for sp in spans {
            var a: [NSAttributedString.Key: Any] = [.font: font(sp.font)]
            if sp.font.tracking != 0 { a[.kern] = CGFloat(sp.font.tracking) }
            if let c = color(sp.color) { a[.foregroundColor] = c }
            out.append(NSAttributedString(string: sp.text, attributes: a))
        }
        if base.lineHeight > 0, let extra = extraAttributes(base), let p = extra[.paragraphStyle] {
            out.addAttribute(.paragraphStyle, value: p, range: NSRange(location: 0, length: out.length))
        }
        return out
    }

    /// Letter spacing and line height, as text attributes; nil when the font
    /// asks for neither (plain text is cheaper).
    static func extraAttributes(_ f: Render.Font) -> [NSAttributedString.Key: Any]? {
        guard f.tracking != 0 || f.lineHeight > 0 else { return nil }
        var a: [NSAttributedString.Key: Any] = [.font: font(f)]
        if f.tracking != 0 { a[.kern] = CGFloat(f.tracking) }
        if f.lineHeight > 0 {
            let p = NSMutableParagraphStyle()
            p.minimumLineHeight = CGFloat(f.lineHeight)
            p.maximumLineHeight = CGFloat(f.lineHeight)
            p.lineBreakMode = .byTruncatingTail
            a[.paragraphStyle] = p
            // Centre the glyphs in the taller (or shorter) line.
            a[.baselineOffset] = (CGFloat(f.lineHeight) - font(f).lineHeight) / 4
        }
        return a
    }

    static func font(_ f: Render.Font) -> UIFont {
        let base = plainFont(f)
        guard f.tabular else { return base }
        // Every digit the same width.
        let desc = base.fontDescriptor.addingAttributes([.featureSettings: [[
            UIFontDescriptor.FeatureKey.type: kNumberSpacingType,
            UIFontDescriptor.FeatureKey.selector: kMonospacedNumbersSelector,
        ]]])
        return UIFont(descriptor: desc, size: base.pointSize)
    }

    private static func plainFont(_ f: Render.Font) -> UIFont {
        let weights: [UIFont.Weight] = [.ultraLight, .thin, .light, .regular, .medium, .semibold, .bold, .heavy, .black]
        let w = f.weight == 0 ? 400 : Int(f.weight)
        let weight = weights[max(0, min(8, w / 100 - 1))]
        if !f.family.isEmpty, let family = Fonts.families[f.family] {
            let desc = UIFontDescriptor(fontAttributes: [.family: family,
                                                         .traits: [UIFontDescriptor.TraitKey.weight: weight]])
            return UIFont(descriptor: desc, size: CGFloat(f.size))
        }
        return .systemFont(ofSize: CGFloat(f.size), weight: weight)
    }
}

/// The fonts the app bundles (`nimbus native font add` puts them in
/// Runner/Fonts), registered at first use: by file name (what Style.FontFamily
/// names) to the family name inside the file.
enum Fonts {
    static let families: [String: String] = {
        var map: [String: String] = [:]
        let urls = (Bundle.main.urls(forResourcesWithExtension: "ttf", subdirectory: nil) ?? [])
            + (Bundle.main.urls(forResourcesWithExtension: "otf", subdirectory: nil) ?? [])
        for url in urls {
            CTFontManagerRegisterFontsForURL(url as CFURL, .process, nil)
            if let descs = CTFontManagerCreateFontDescriptorsFromURL(url as CFURL) as? [CTFontDescriptor],
               let family = descs.first.flatMap({ CTFontDescriptorCopyAttribute($0, kCTFontFamilyNameAttribute) as? String }) {
                map[url.deletingPathExtension().lastPathComponent] = family
            }
        }
        return map
    }()
}

/// A canvas (ui.Canvas): Go's shapes, drawn with Core Graphics in a W×H space
/// fitted to the view.
final class NNCanvas: UIView {
    private var d = Render.Draw()

    override init(frame: CGRect) {
        super.init(frame: frame)
        isOpaque = false
        backgroundColor = .clear
        contentMode = .redraw
        isUserInteractionEnabled = false
    }
    required init?(coder: NSCoder) { fatalError() }

    func set(_ o: Render.Draw) {
        d = o
        setNeedsDisplay()
    }

    override func draw(_ rect: CGRect) {
        guard let c = UIGraphicsGetCurrentContext(), d.w > 0, d.h > 0 else { return }
        let s = min(bounds.width / CGFloat(d.w), bounds.height / CGFloat(d.h))
        c.translateBy(x: (bounds.width - CGFloat(d.w) * s) / 2, y: (bounds.height - CGFloat(d.h) * s) / 2)
        c.scaleBy(x: s, y: s)
        var n = 0, k = 0
        func num() -> CGFloat { defer { n += 1 }; return n < d.nums.count ? CGFloat(d.nums[n]) : 0 }
        func col() -> CGColor { defer { k += 1 }; return (k < d.colors.count ? color(d.colors[k]) : nil)?.cgColor ?? UIColor.clear.cgColor }
        var path = CGMutablePath()
        for cmd in d.cmds {
            switch cmd {
            case 0: path.move(to: CGPoint(x: num(), y: num()))
            case 1: path.addLine(to: CGPoint(x: num(), y: num()))
            case 2: let cx = num(), cy = num(); path.addQuadCurve(to: CGPoint(x: num(), y: num()), control: CGPoint(x: cx, y: cy))
            case 3:
                let a = CGPoint(x: num(), y: num()), b = CGPoint(x: num(), y: num())
                path.addCurve(to: CGPoint(x: num(), y: num()), control1: a, control2: b)
            case 4: path.closeSubpath()
            case 5: let x = num(), y = num(), r = num(); path.addEllipse(in: CGRect(x: x - r, y: y - r, width: r * 2, height: r * 2))
            case 6:
                let r = CGRect(x: num(), y: num(), width: num(), height: num()), rad = min(num(), min(r.width, r.height) / 2)
                path.addRoundedRect(in: r, cornerWidth: max(rad, 0), cornerHeight: max(rad, 0))
            case 7:
                c.addPath(path); c.setFillColor(col()); c.fillPath()
                path = CGMutablePath()
            case 8:
                let w = num()
                c.addPath(path); c.setStrokeColor(col()); c.setLineWidth(w); c.setLineCap(.round); c.setLineJoin(.round); c.strokePath()
                path = CGMutablePath()
            case 9:
                let radial = num() != 0
                let a = CGPoint(x: num(), y: num()), b = CGPoint(x: num(), y: num())
                let count = Int(num())
                var stops: [CGFloat] = [], cols: [CGColor] = []
                for _ in 0..<max(count, 0) { stops.append(num()) }
                for _ in 0..<max(count, 0) { cols.append(col()) }
                if count > 0, let g = CGGradient(colorsSpace: CGColorSpaceCreateDeviceRGB(), colors: cols as CFArray, locations: stops) {
                    c.saveGState()
                    c.addPath(path)
                    c.clip()
                    if radial {
                        c.drawRadialGradient(g, startCenter: a, startRadius: 0, endCenter: a, endRadius: hypot(b.x - a.x, b.y - a.y),
                                             options: [.drawsAfterEndLocation])
                    } else {
                        c.drawLinearGradient(g, start: a, end: b, options: [.drawsBeforeStartLocation, .drawsAfterEndLocation])
                    }
                    c.restoreGState()
                }
                path = CGMutablePath()
            default: return // a newer command: stop rather than draw it wrong
            }
        }
    }
}

/// The blur under a view's children (Style.Blur): kept first among them.
final class NNBackdrop: UIVisualEffectView {
    override func hitTest(_ point: CGPoint, with event: UIEvent?) -> UIView? { nil }
}

/// What a box draws besides its background: a blur of what is behind it, a
/// gradient and a shadow. Laid out again whenever the view's frame changes.
final class Decor {
    var blur: NNBackdrop?
    var glass = false // Liquid Glass: no tint over it
    var gradient: CAGradientLayer?
    private var shadow = false
    private var corners: UInt8 = 0

    var isEmpty: Bool { blur == nil && gradient == nil && !shadow }

    /// Above the blur, under everything else.
    func keepGradientBelow(_ v: UIView) {
        guard let gradient else { return }
        if let blur { v.layer.insertSublayer(gradient, above: blur.layer) } else { v.layer.insertSublayer(gradient, at: 0) }
    }

    static func corners(_ mask: UInt8) -> CACornerMask {
        guard mask != 0 else { return [.layerMinXMinYCorner, .layerMaxXMinYCorner, .layerMaxXMaxYCorner, .layerMinXMaxYCorner] }
        var m: CACornerMask = []
        if mask & 1 != 0 { m.insert(.layerMinXMinYCorner) }
        if mask & 2 != 0 { m.insert(.layerMaxXMinYCorner) }
        if mask & 4 != 0 { m.insert(.layerMaxXMaxYCorner) }
        if mask & 8 != 0 { m.insert(.layerMinXMaxYCorner) }
        return m
    }

    func apply(_ o: Render.Box, to v: UIView) {
        corners = o.corners
        // Blur: the system materials.
        let style: UIBlurEffect.Style? = switch o.blur {
        case .ultraThin, .glass: .systemUltraThinMaterial
        case .thin: .systemThinMaterial
        case .regular: .systemMaterial
        case .thick: .systemThickMaterial
        default: nil
        }
        if let style {
            if blur == nil {
                let b = NNBackdrop(effect: nil)
                b.clipsToBounds = true
                v.insertSubview(b, at: 0)
                blur = b
            }
            glass = false
            if o.blur == .glass, #available(iOS 26.0, *) {
                let g = UIGlassEffect(style: .regular)
                g.isInteractive = true
                blur?.effect = g
                glass = true
            } else {
                blur?.effect = UIBlurEffect(style: style)
            }
        } else if let b = blur {
            b.removeFromSuperview()
            blur = nil
        }

        // Gradient, over the background and under the children.
        if o.gradientColors.count >= 2 {
            let g = gradient ?? CAGradientLayer()
            if gradient == nil { v.layer.insertSublayer(g, at: 0); gradient = g }
            g.colors = o.gradientColors.map { (color($0) ?? .clear).cgColor }
            g.locations = o.gradientStops.count == o.gradientColors.count ? o.gradientStops.map { NSNumber(value: $0) } : nil
            g.type = o.gradientRadial ? .radial : .axial
            g.startPoint = CGPoint(x: CGFloat(o.gradientX0), y: CGFloat(o.gradientY0))
            if o.gradientRadial {
                // Radial: the end point sets the radius, in both directions.
                let r = hypot(CGFloat(o.gradientX1 - o.gradientX0), CGFloat(o.gradientY1 - o.gradientY0))
                g.endPoint = CGPoint(x: CGFloat(o.gradientX0) + r, y: CGFloat(o.gradientY0) + r)
            } else {
                g.endPoint = CGPoint(x: CGFloat(o.gradientX1), y: CGFloat(o.gradientY1))
            }
        } else if let g = gradient {
            g.removeFromSuperlayer()
            gradient = nil
        }

        // Shadow, on the view's own layer.
        shadow = o.shadowColor != 0
        v.layer.shadowColor = color(o.shadowColor)?.cgColor
        v.layer.shadowOpacity = shadow ? 1 : 0
        v.layer.shadowRadius = CGFloat(o.shadowRadius) / 2 // CSS-like: the blur's extent
        v.layer.shadowOffset = CGSize(width: CGFloat(o.shadowX), height: CGFloat(o.shadowY))
        if !shadow { v.layer.shadowPath = nil }
    }

    func layout(_ v: UIView) {
        let r = v.layer.cornerRadius
        let masked = v.layer.maskedCorners
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        if let b = blur {
            b.frame = v.bounds
            b.layer.cornerRadius = r
            b.layer.maskedCorners = masked
            if glass, #available(iOS 26.0, *) {
                // Liquid Glass follows the view's corner configuration, not its layer mask.
                func at(_ c: CACornerMask) -> UICornerRadius { .fixed(masked.contains(c) ? Double(r) : 0) }
                b.cornerConfiguration = .corners(topLeftRadius: at(.layerMinXMinYCorner), topRightRadius: at(.layerMaxXMinYCorner),
                                                 bottomLeftRadius: at(.layerMinXMaxYCorner), bottomRightRadius: at(.layerMaxXMaxYCorner))
            }
        }
        if let g = gradient {
            g.frame = v.bounds
            g.cornerRadius = r
            g.maskedCorners = masked
        }
        if shadow {
            var rc: UIRectCorner = []
            if masked.contains(.layerMinXMinYCorner) { rc.insert(.topLeft) }
            if masked.contains(.layerMaxXMinYCorner) { rc.insert(.topRight) }
            if masked.contains(.layerMaxXMaxYCorner) { rc.insert(.bottomRight) }
            if masked.contains(.layerMinXMaxYCorner) { rc.insert(.bottomLeft) }
            v.layer.shadowPath = UIBezierPath(roundedRect: v.bounds, byRoundingCorners: rc, cornerRadii: CGSize(width: r, height: r)).cgPath
        }
        CATransaction.commit()
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
private func gesturePhase(_ s: UIGestureRecognizer.State) -> UInt8? {
    switch s {
    case .began: return 0
    case .changed: return 1
    case .ended: return 2
    case .cancelled, .failed: return 3
    default: return nil
    }
}

/// Reports drags, measured in the window so a view that follows the finger
/// doesn't change them. A one-way drag only starts going that way.
final class NNPan: UIPanGestureRecognizer, UIGestureRecognizerDelegate {
    var axis: UInt8 = 1
    private let report: (UInt8, CGPoint, CGPoint) -> Void

    init(_ report: @escaping (UInt8, CGPoint, CGPoint) -> Void) {
        self.report = report
        super.init(target: nil, action: nil)
        addTarget(self, action: #selector(moved))
        delegate = self
    }

    func gestureRecognizerShouldBegin(_ g: UIGestureRecognizer) -> Bool {
        let v = velocity(in: view?.window)
        switch axis {
        case 2: return abs(v.x) > abs(v.y)
        case 3: return abs(v.y) > abs(v.x)
        default: return true
        }
    }

    @objc private func moved() {
        guard let phase = gesturePhase(state) else { return }
        report(phase, translation(in: view?.window), velocity(in: view?.window))
    }
}

/// Reports two-finger pinches.
final class NNPinch: UIPinchGestureRecognizer {
    private let report: (UInt8, Float) -> Void

    init(_ report: @escaping (UInt8, Float) -> Void) {
        self.report = report
        super.init(target: nil, action: nil)
        addTarget(self, action: #selector(moved))
    }

    @objc private func moved() {
        guard let phase = gesturePhase(state) else { return }
        report(phase, Float(scale))
    }
}

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

    private(set) var reportsLongPress = false
    private var onLongPress: (() -> Void)?

    /// Reports long presses (and keeps a long press from also being a tap).
    func reportLongPress(_ f: @escaping () -> Void) {
        guard !reportsLongPress else { return }
        reportsLongPress = true
        onLongPress = f
        addGestureRecognizer(UILongPressGestureRecognizer(target: self, action: #selector(longPressed(_:))))
    }

    @objc private func longPressed(_ g: UILongPressGestureRecognizer) {
        guard g.state == .began else { return }
        cancelTracking(with: nil) // the touch is a long press now, not a tap
        isHighlighted = false
        onLongPress?()
    }
}

/// A text field with room around the text (the runtime measures inputs with
/// 10 pt of padding).
/// A text input: a UITextField, or a UITextView once Go asks for several
/// lines. Border, background and corners are on this view either way.
final class NNInput: UIView, UITextViewDelegate {
    private let pad: CGFloat = 10
    private let onChange: (String) -> Void
    var onSubmit: (() -> Void)?
    private var submits = false
    private var focusedOnce = false
    private lazy var field: UITextField = {
        let f = UITextField()
        f.addAction(UIAction { [weak self, weak f] _ in self?.onChange(f?.text ?? "") }, for: .editingChanged)
        // Return: send, keeping the keyboard up (a chat goes on).
        f.addAction(UIAction { [weak self] _ in if self?.submits == true { self?.onSubmit?() } }, for: .primaryActionTriggered)
        return f
    }()
    private var area: UITextView?
    private let hint = UILabel() // the text view's placeholder
    private var multiline = false

    init(onChange: @escaping (String) -> Void) {
        self.onChange = onChange
        super.init(frame: .zero)
        addSubview(field)
    }

    required init?(coder: NSCoder) { fatalError("not used") }

    var text: String {
        multiline ? area?.text ?? "" : field.text ?? ""
    }

    func apply(_ o: Render.Input) {
        if o.multiline != multiline { switchMode(o.multiline) }
        let font = Renderer.font(o.font), ink = color(o.color) ?? .label
        if let area {
            if area.text != o.value { area.text = o.value }
            area.font = font
            area.textColor = ink
            hint.text = o.placeholder
            hint.font = font
            hint.isHidden = !o.value.isEmpty
        } else {
            if field.text != o.value { field.text = o.value }
            field.placeholder = o.placeholder.isEmpty ? nil : o.placeholder
            field.font = font
            field.textColor = ink
            field.isSecureTextEntry = o.secure
        }
        let type: UIKeyboardType
        switch o.keyboard {
        case .number: type = .decimalPad
        case .email: type = .emailAddress
        case .url: type = .URL
        default: type = .default
        }
        field.keyboardType = type
        area?.keyboardType = type
        submits = o.submit
        field.returnKeyType = o.submit ? .send : .default
        area?.returnKeyType = o.submit ? .send : .default
        if o.autoFocus && !focusedOnce {
            focusedOnce = true
            // Once on screen: focusing a view outside a window does nothing.
            DispatchQueue.main.async { [weak self] in
                guard let self, self.window != nil else { return }
                _ = (self.area as UIView?)?.becomeFirstResponder() ?? self.field.becomeFirstResponder()
            }
        }
    }

    /// Return in a text view that submits sends instead of breaking the line.
    func textView(_ t: UITextView, shouldChangeTextIn range: NSRange, replacementText s: String) -> Bool {
        if submits, s == "\n" { onSubmit?(); return false }
        return true
    }

    private func switchMode(_ multi: Bool) {
        multiline = multi
        if multi {
            field.removeFromSuperview()
            let a = UITextView()
            a.backgroundColor = .clear
            a.textContainerInset = UIEdgeInsets(top: pad, left: pad - 5, bottom: pad, right: pad - 5)
            a.delegate = self
            hint.textColor = .placeholderText
            hint.numberOfLines = 1
            addSubview(a)
            addSubview(hint)
            area = a
        } else {
            area?.removeFromSuperview()
            hint.removeFromSuperview()
            area = nil
            addSubview(field)
        }
        setNeedsLayout()
    }

    func textViewDidChange(_ t: UITextView) {
        hint.isHidden = !t.text.isEmpty
        onChange(t.text)
    }

    /// What the renderer's type(tag:text:) does: set the text and report it.
    func userTyped(_ s: String) {
        if let area {
            area.text = s
            textViewDidChange(area)
        } else {
            field.text = s
            field.sendActions(for: .editingChanged)
        }
    }

    override func layoutSubviews() {
        super.layoutSubviews()
        field.frame = bounds.insetBy(dx: pad, dy: 0)
        area?.frame = bounds
        hint.frame = CGRect(x: pad, y: pad, width: bounds.width - 2 * pad, height: hint.font.lineHeight)
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

    /// Closes the sheet as the user would, and tells Go.
    func close() {
        dismiss(animated: true)
        renderer?.send(.dismissed(Render.Dismissed(tag: modalTag)))
    }
}
