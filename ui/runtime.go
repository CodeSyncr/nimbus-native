// Package ui is Nimbus Native's component layer: apps describe their screens
// as Go components, and the runtime keeps real native views (UIKit, Android
// Views) in step with them.
//
// Everything happens on one goroutine, the UI loop: host events, state
// updates, rendering, layout and effects. A render reconciles the new
// element tree against the one on screen, lays it out with the layout
// package, and sends the differences to the host as one render.Commit.
package ui

import (
	"os"
	"reflect"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/CodeSyncr/nimbus-native/bridge"
	"github.com/CodeSyncr/nimbus-native/layout"
	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Mount makes root the app's UI. Call it from init; the UI loop starts when
// the host starts the runtime.
func Mount(root Component) {
	rt := newRuntime(root)
	rt.send = func(b []byte) { bridge.Send(b) }
	rt.measure = func(m *render.MeasureText) render.TextSize {
		reply, err := bridge.CallHost(render.Encode(m))
		if err == nil {
			if r, err := render.Decode(reply); err == nil {
				if ts, ok := r.(*render.TextSize); ok {
					return *ts
				}
			}
		}
		logf("text measurement failed: %v", err)
		return render.TextSize{}
	}
	logf = bridge.Logf
	if f := os.Getenv("NIMBUS_DEV_STATE"); f != "" {
		rt.loadNav(f)
	}
	bridge.OnMessage(rt.receive)
	bridge.OnStart(rt.loop)
}

// Main is for the app's main function: func main() { ui.Main() }. On a device
// the host owns the process and main never runs; under `nimbus native dev`
// the app runs on the laptop and Main serves the device until it reloads.
func Main() { bridge.Main() }

type runtime struct {
	mu    sync.Mutex
	queue []func()
	wake  chan struct{}

	// Ports to the host; tests replace them.
	send    func([]byte)
	measure func(*render.MeasureText) render.TextSize

	// Owned by the UI loop.
	root         Component
	rootInst     *instance
	rootTag      uint32
	viewport     ViewportInfo
	haveViewport bool
	dirty        bool
	forceAll     bool // re-render every component (the viewport changed)
	nextTag      uint32
	byTag        map[uint32]*instance
	ops          []render.Op
	destroyed    []uint32
	effects      []*hookSlot
	sentChrome   render.Chrome // the theme the host last got
	queries      map[string]*queryEntry
	modals       []*modal // open modals, bottom first
	modalKey     int
	lists        map[*instance]bool // mounted Lists
	rechild      map[*instance]bool // Lists and Stacks whose children must render again
	toggled      map[*instance]bool // Switches the user flipped, to check against Go's answer
	seq          uint64
	measures     uint32

	// Stores (store.go).
	storesMu sync.Mutex
	stores   map[any]*storeState

	// Sessions (session.go).
	hostLayout func(tag uint32, s layout.Style)
	values     map[any]any
	closed     bool

	// Dev reloads (navstate.go).
	navFile            string
	navSaved           *navState
	navLast            string
	navTabs, navStacks int
}

func newRuntime(root Component) *runtime {
	return &runtime{root: root, wake: make(chan struct{}, 1), byTag: map[uint32]*instance{},
		lists: map[*instance]bool{}, rechild: map[*instance]bool{}, toggled: map[*instance]bool{}, queries: map[string]*queryEntry{},
		stores: map[any]*storeState{}}
}

// do runs f on the UI loop. Safe from any goroutine, including the loop.
func (rt *runtime) do(f func()) {
	rt.mu.Lock()
	if rt.closed {
		rt.mu.Unlock()
		return
	}
	rt.queue = append(rt.queue, f)
	rt.mu.Unlock()
	select {
	case rt.wake <- struct{}{}:
	default:
	}
}

func (rt *runtime) loop() {
	for range rt.wake {
		rt.drain()
		if rt.closed {
			return
		}
	}
}

// drain runs queued work until none is left, then renders if anything
// changed: many state updates in a row cost one render.
func (rt *runtime) drain() {
	for {
		rt.mu.Lock()
		q := rt.queue
		rt.queue = nil
		rt.mu.Unlock()
		if len(q) == 0 {
			break
		}
		for _, f := range q {
			guard("update", f)
		}
	}
	if rt.closed {
		return
	}
	// A List measures its items after layout and may need another pass to
	// place them; a few passes always settle.
	for pass := 0; rt.dirty && rt.haveViewport && pass < 4; pass++ {
		rt.commit()
	}
}

// receive handles a message from the host (on the bridge's goroutine).
func (rt *runtime) receive(b []byte) {
	m, err := render.Decode(b)
	if err != nil {
		logf("ui: undecodable host message: %v", err)
		return
	}
	if r, ok := m.(*render.PluginResult); ok {
		plugin.Deliver(r) // straight to the waiting call, not through the UI loop
		return
	}
	rt.do(func() {
		switch m := m.(type) {
		case *render.Viewport:
			rt.viewport = ViewportInfo{W: m.W, H: m.H, Scale: m.Scale, SafeTop: m.SafeTop, SafeBottom: m.SafeBottom,
				SafeLeft: m.SafeLeft, SafeRight: m.SafeRight, Platform: m.Platform, Dark: m.Dark, Keyboard: m.Keyboard}
			rt.haveViewport, rt.dirty, rt.forceAll = true, true, true
		case *render.Press:
			if in := rt.byTag[m.Tag]; in != nil && in.el.onPress != nil {
				in.el.onPress()
			}
		case *render.Changed:
			if in := rt.byTag[m.Tag]; in != nil && in.el.onChange != nil {
				in.el.onChange(m.Text)
			}
		case *render.Scrolled:
			if in := rt.byTag[m.Tag]; in != nil {
				rt.scrolled(in, m)
			}
		case *render.Popped:
			if in := rt.byTag[m.Tag]; in != nil {
				rt.popped(in, int(m.Depth))
			}
		case *render.Selected:
			if in := rt.byTag[m.Tag]; in != nil {
				rt.selected(in, int(m.Index))
			}
		case *render.Resized:
			if in := rt.byTag[m.Tag]; in != nil && !rt.modalResized(in, m.W, m.H) {
				rt.resized(in, m.W, m.H, m.Bottom)
			}
		case *render.Dismissed:
			rt.dismissed(m.Tag)
		case *render.Toggled:
			if in := rt.byTag[m.Tag]; in != nil {
				in.sent.toggle.On = m.On // what the host shows; Go's answer is diffed against it
				if in.el.onToggle != nil {
					in.el.onToggle(m.On)
				}
				rt.toggled[in] = true
				rt.dirty = true
			}
		case *render.Refresh:
			if in := rt.byTag[m.Tag]; in != nil {
				in.sent.scroll.Refreshing = true // the host is spinning until Go says otherwise
				if in.el.onRefresh != nil {
					in.el.onRefresh()
				}
				rt.dirty = true
			}
		}
	})
}

// instance is an element on screen: a component with its hooks, or a native
// view with its tag, layout node and the last props sent for it.
type instance struct {
	el        *Element
	parent    *instance
	unmounted bool
	dirty     bool // a component whose state changed since its last render

	// Component
	hooks []*hookSlot
	child *instance

	// Native view
	tag       uint32
	node      *layout.Node
	children  []*instance
	childTags []uint32
	sent      sentProps
	title     string // a stack screen's or tab's title, as last sent
	icon      string // a tab's icon, as last sent
	measured  map[measureKey]render.TextSize
	list      *listState  // for a List
	stack     *stackState // for a Stack
	tabs      *tabsState  // for Tabs
}

type sentProps struct {
	box    render.Box
	text   render.Text
	image  render.Image
	input  render.Input
	scroll render.Scroll
	spin   render.Spinner
	toggle render.Switch
	frame  render.Frame
}

type measureKey struct {
	w    float32
	mode layout.Mode
}

// commit renders the whole tree and sends what changed.
func (rt *runtime) commit() {
	t0 := time.Now()
	rt.dirty, rt.measures = false, 0
	rt.chrome()
	rt.rootInst = rt.reconcile(rt.rootInst, C(rt.root), nil)
	rt.reconcileModals()
	rt.forceAll = false
	for in := range rt.rechild {
		if !in.unmounted {
			rt.children(in)
		}
		delete(rt.rechild, in)
	}
	// A flipped switch whose handler kept the old state goes back.
	for in := range rt.toggled {
		if !in.unmounted && in.sent.toggle.On != in.el.on {
			sw := in.sent.toggle
			sw.On = in.el.on
			in.sent.toggle = sw
			rt.op(&sw)
		}
		delete(rt.toggled, in)
	}
	c := &render.Commit{}
	host := hostRoot(rt.rootInst)
	if host != nil && host.tag != rt.rootTag {
		rt.rootTag, c.Root = host.tag, host.tag
	}
	t1 := time.Now()
	if host != nil && rt.hostLayout == nil {
		host.node.Calculate(rt.viewport.W, rt.viewport.H)
		if rt.measureLists() {
			rt.dirty = true // drain renders again with the measured heights
		}
		rt.emitFrames(host)
	}
	if rt.hostLayout == nil {
		rt.layoutModals()
	}
	t2 := time.Now()
	if len(rt.destroyed) > 0 {
		rt.ops = append(rt.ops, &render.Destroy{Tags: rt.destroyed})
		rt.destroyed = nil
	}
	rt.seq++
	c.Seq, c.Ops = rt.seq, rt.ops
	c.BuildNs, c.LayoutNs, c.Measures = uint64(t1.Sub(t0)), uint64(t2.Sub(t1)), rt.measures
	rt.send(render.Encode(c))
	rt.ops = rt.ops[:0]

	rt.saveNav()

	effects := rt.effects
	rt.effects = nil
	for _, s := range effects {
		if s.pending {
			s.pending = false
			s.start()
		}
	}
}

func (rt *runtime) op(o render.Op) { rt.ops = append(rt.ops, o) }

// reconcile makes old show el, reusing it when it is the same type.
func (rt *runtime) reconcile(old *instance, el *Element, parent *instance) *instance {
	switch {
	case el == nil:
		if old != nil {
			rt.unmount(old)
		}
		return nil
	case old == nil:
		return rt.mount(el, parent)
	case !el.sameType(old.el):
		rt.unmount(old)
		return rt.mount(el, parent)
	}
	rt.update(old, el)
	return old
}

func (rt *runtime) mount(el *Element, parent *instance) *instance {
	in := &instance{el: el, parent: parent}
	if el.comp != nil {
		in.child = rt.reconcile(nil, rt.renderComponent(in), in)
		return in
	}
	rt.nextTag++
	in.tag = rt.nextTag
	in.node = &layout.Node{}
	rt.byTag[in.tag] = in
	rt.op(&render.Create{Tag: in.tag, Kind: el.kind})
	switch el.kind {
	case render.KindText:
		in.node.Measure = rt.textMeasure(in)
	case render.KindInput:
		in.node.Measure = rt.inputMeasure(in)
	case render.KindSpinner:
		in.node.Measure = rt.spinnerMeasure
	case render.KindSwitch:
		in.node.Measure = rt.switchMeasure
	}
	rt.props(in, true)
	rt.children(in)
	return in
}

func (rt *runtime) update(in *instance, el *Element) {
	if el.comp != nil {
		// Same props and no state change: what it rendered last time stands.
		skip := !rt.forceAll && !in.dirty && propsEqual(in.el.comp, el.comp)
		in.el = el
		if skip {
			rt.refresh(in.child) // components further down may have changed state
		} else {
			in.dirty = false
			in.child = rt.reconcile(in.child, rt.renderComponent(in), in)
		}
		return
	}
	in.el = el
	rt.props(in, false)
	rt.children(in)
}

// refresh walks a subtree whose elements are unchanged and re-renders the
// components in it whose state changed.
func (rt *runtime) refresh(in *instance) {
	if in == nil {
		return
	}
	if in.el.comp != nil {
		if in.dirty {
			in.dirty = false
			in.child = rt.reconcile(in.child, rt.renderComponent(in), in)
		} else {
			rt.refresh(in.child)
		}
		return
	}
	for _, c := range in.children {
		rt.refresh(c)
	}
	rt.syncChildren(in) // a re-rendered component may now show a different view
}

// children reconciles a native view's children: its elements, or for a
// List the window of items to show.
func (rt *runtime) children(in *instance) {
	switch {
	case in.el.list != nil:
		rt.reconcileList(in)
		return
	case in.el.stack != nil:
		rt.reconcileStack(in)
		return
	case in.el.tabs != nil:
		rt.reconcileTabs(in)
		return
	}
	rt.reconcileChildren(in, in.el.children)
}

func (rt *runtime) renderComponent(in *instance) (out *Element) {
	defer func() {
		if r := recover(); r != nil {
			logf("panic rendering %T: %v\n%s", in.el.comp, r, debug.Stack())
			out = nil
		}
	}()
	out = in.el.comp.Render(&Ctx{rt: rt, inst: in})
	// A component's TestID and Label land on what it renders, unless that
	// sets its own: ui.C(Button{...}).TestID("save") names the button.
	if out != nil && (in.el.testID != "" && out.testID == "" || in.el.label != "" && out.label == "") {
		cp := *out // the component may return a shared element
		if cp.testID == "" {
			cp.testID = in.el.testID
		}
		if cp.label == "" {
			cp.label = in.el.label
		}
		out = &cp
	}
	return out
}

// reconcileChildren matches new child elements to existing instances by key
// (or position), then tells the host the native child order if it changed.
func (rt *runtime) reconcileChildren(in *instance, els []*Element) {
	old := make(map[string]*instance, len(in.children))
	for i, c := range in.children {
		old[childKey(c.el, i)] = c
	}
	next := make([]*instance, 0, len(els))
	for i, el := range els {
		if el == nil {
			continue
		}
		k := childKey(el, i)
		if o := old[k]; o != nil && el.sameType(o.el) {
			delete(old, k)
			rt.update(o, el)
			next = append(next, o)
		} else {
			next = append(next, rt.mount(el, in))
		}
	}
	for _, o := range old {
		rt.unmount(o)
	}
	in.children = next
	rt.syncChildren(in)
}

// syncChildren tells the host (and the layout) a view's native children
// when they changed.
func (rt *runtime) syncChildren(in *instance) {
	tags := make([]uint32, 0, len(in.children))
	for _, c := range in.children {
		if h := hostRoot(c); h != nil {
			tags = append(tags, h.tag)
		}
	}
	if !equalTags(tags, in.childTags) {
		// Tags name instances one to one, so the layout children change
		// exactly when the tags do.
		nodes := make([]*layout.Node, 0, len(tags))
		for _, t := range tags {
			nodes = append(nodes, rt.byTag[t].node)
		}
		in.node.Children = nodes
		in.node.MarkDirty()
		in.childTags = tags
		rt.op(&render.Children{Tag: in.tag, Children: tags})
	}
}

// propsEqual reports whether two components have equal props. Components
// with props Go can't compare (slices, maps, funcs) always re-render.
func propsEqual(a, b Component) (eq bool) {
	t := reflect.TypeOf(a)
	if t != reflect.TypeOf(b) || !t.Comparable() {
		return false
	}
	defer func() {
		if recover() != nil {
			eq = false // an interface field holding something incomparable
		}
	}()
	return a == b
}

func childKey(el *Element, i int) string {
	if el.key != "" {
		return "k" + el.key
	}
	return "i" + strconv.Itoa(i)
}

func (rt *runtime) unmount(in *instance) {
	in.unmounted = true
	for _, s := range in.hooks {
		s.stop()
		s.pending = false
	}
	if in.child != nil {
		rt.unmount(in.child)
	}
	for _, c := range in.children {
		rt.unmount(c)
	}
	if in.tag != 0 {
		delete(rt.byTag, in.tag)
		rt.destroyed = append(rt.destroyed, in.tag)
	}
	delete(rt.lists, in)
	delete(rt.rechild, in)
}

// hostRoot is the native view an instance renders to.
func hostRoot(in *instance) *instance {
	for in != nil && in.tag == 0 {
		in = in.child
	}
	return in
}

// props sends whatever of el's look changed since the last commit.
func (rt *runtime) props(in *instance, first bool) {
	el, s := in.el, &in.el.style
	ls := s.layout()
	ls.Scroll = el.kind == render.KindScroll
	if ls != in.node.Style || (first && rt.hostLayout != nil) {
		in.node.Style = ls
		in.node.MarkDirty()
		if rt.hostLayout != nil {
			rt.hostLayout(in.tag, ls)
		}
	}

	box := render.Box{Tag: in.tag, Background: uint32(s.Background), Radius: s.Radius, BorderWidth: s.BorderWidth,
		BorderColor: uint32(s.BorderColor), Opacity: s.opacity(), Clip: s.Clip, Label: el.label, TestID: el.testID, Href: el.href}
	if box != in.sent.box && !(first && box == (render.Box{Tag: in.tag, Opacity: 1})) {
		rt.op(&box)
	}
	in.sent.box = box

	font := render.Font{Size: s.fontSize(), Weight: s.FontWeight}
	theme := rt.theme()
	textColor := s.Color
	if textColor == 0 {
		textColor = theme.Text
	}
	switch el.kind {
	case render.KindText:
		t := render.Text{Tag: in.tag, Text: el.text, Font: font, Color: uint32(textColor), Align: textAlign(s.TextAlign), Lines: s.Lines}
		if first || t != in.sent.text {
			if t.Text != in.sent.text.Text || t.Font != in.sent.text.Font || t.Lines != in.sent.text.Lines {
				in.measured = nil
				in.node.MarkDirty()
			}
			in.sent.text = t
			rt.op(&t)
		}
	case render.KindImage:
		img := render.Image{Tag: in.tag, URL: el.src, Fit: el.fit}
		if first || img != in.sent.image {
			in.sent.image = img
			rt.op(&img)
		}
	case render.KindInput:
		inp := render.Input{Tag: in.tag, Value: el.text, Placeholder: el.placeholder, Font: font,
			Color: uint32(textColor), Secure: el.secure, Keyboard: el.keyboard, Multiline: el.multiline}
		if first || inp != in.sent.input {
			if inp.Font != in.sent.input.Font || inp.Multiline != in.sent.input.Multiline {
				in.measured = nil
				in.node.MarkDirty()
			}
			in.sent.input = inp
			rt.op(&inp)
		}
	case render.KindSpinner:
		spin := s.Color
		if spin == 0 {
			spin = theme.Primary
		}
		sp := render.Spinner{Tag: in.tag, Color: uint32(spin)}
		if first || sp != in.sent.spin {
			in.sent.spin = sp
			rt.op(&sp)
		}
	case render.KindSwitch:
		tint := s.Color
		if tint == 0 {
			tint = theme.Primary
		}
		sw := render.Switch{Tag: in.tag, On: el.on, Enabled: el.onToggle != nil, Tint: uint32(tint)}
		if first || sw != in.sent.toggle {
			in.sent.toggle = sw
			rt.op(&sw)
		}
	}
}

// emitFrames sends the frames (and scroll content sizes) layout changed.
func (rt *runtime) emitFrames(in *instance) {
	n := in.node
	f := render.Frame{Tag: in.tag, X: n.X, Y: n.Y, W: n.W, H: n.H}
	if f != in.sent.frame {
		in.sent.frame = f
		rt.op(&f)
	}
	if in.el.kind == render.KindScroll {
		sc := render.Scroll{Tag: in.tag, W: n.ContentW, H: n.ContentH, Horizontal: in.el.horizontal, Report: in.el.list != nil,
			Refreshable: in.el.onRefresh != nil, Refreshing: in.el.onRefresh != nil && in.el.refreshing}
		if sc != in.sent.scroll {
			in.sent.scroll = sc
			rt.op(&sc)
		}
	}
	for _, c := range in.children {
		if h := hostRoot(c); h != nil {
			rt.emitFrames(h)
		}
	}
}

func (rt *runtime) textMeasure(in *instance) layout.MeasureFunc {
	return func(w float32, wm layout.Mode, h float32, hm layout.Mode) (float32, float32) {
		if wm == layout.Undefined {
			w = -1
		}
		key := measureKey{w, wm}
		if sz, ok := in.measured[key]; ok {
			return sz.W, sz.H
		}
		t := in.sent.text
		rt.measures++
		sz := rt.measure(&render.MeasureText{Text: t.Text, Font: t.Font, MaxWidth: w, Lines: t.Lines})
		if in.measured == nil {
			in.measured = map[measureKey]render.TextSize{}
		}
		in.measured[key] = sz
		return sz.W, sz.H
	}
}

// inputMeasure: one line of the input's font plus the platform's field
// padding; as wide as offered.
func (rt *runtime) inputMeasure(in *instance) layout.MeasureFunc {
	return func(w float32, wm layout.Mode, h float32, hm layout.Mode) (float32, float32) {
		key := measureKey{-2, 0}
		sz, ok := in.measured[key]
		if !ok {
			rt.measures++
			sz = rt.measure(&render.MeasureText{Text: "Ag", Font: in.sent.input.Font, MaxWidth: -1})
			if in.measured == nil {
				in.measured = map[measureKey]render.TextSize{}
			}
			in.measured[key] = sz
		}
		width := float32(160)
		if wm != layout.Undefined {
			width = w
		}
		if in.sent.input.Multiline {
			return width, 3*sz.H + 2*fieldPadding
		}
		return width, sz.H + 2*fieldPadding
	}
}

// switchMeasure is the platform toggle's own size.
func (rt *runtime) switchMeasure(w float32, wm layout.Mode, h float32, hm layout.Mode) (float32, float32) {
	if rt.viewport.Platform == "android" {
		return 52, 32
	}
	return 51, 31
}

const fieldPadding = 10

func (rt *runtime) spinnerMeasure(w float32, wm layout.Mode, h float32, hm layout.Mode) (float32, float32) {
	if rt.viewport.Platform == "android" {
		return 32, 32
	}
	return 20, 20
}

func textAlign(a Align) render.Align {
	switch a {
	case AlignCenter:
		return render.AlignCenter
	case AlignEnd:
		return render.AlignEnd
	}
	return render.AlignStart
}

func equalTags(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func guard(what string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			logf("panic in %s: %v\n%s", what, r, debug.Stack())
		}
	}()
	f()
}
