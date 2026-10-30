// Package render is the wire protocol between the ui runtime (Go) and a
// host's native renderer. Its generated Swift and Java ship inside the host
// kits: regenerate with `go test ./ui/render -update` after changing it.
//
// Lengths are points (iOS) / dp (Android). Colors are 0xRRGGBBAA; 0 means
// unset (transparent background, default text color).
package render

// Kind is the native view a tag stands for.
//
//wire:enum
type Kind uint8

const (
	KindView Kind = iota
	KindText
	KindImage
	KindPressable
	KindScroll
	KindInput
	KindSpinner
	// KindStack is a navigation stack: its children are screens, shown one
	// at a time under a native bar, with native push / pop transitions.
	KindStack
	// KindTabs is a tab bar: its children are the tabs' root views, one
	// shown at a time above the native tab bar.
	KindTabs
	// KindModal is a view presented over everything (a sheet on iOS). It is
	// not placed by a Children op: Present shows it, Destroy dismisses it.
	KindModal
	// KindSwitch is an on/off toggle: UISwitch, Android's Switch.
	KindSwitch
	// KindSlider picks a number in a range: UISlider, Android's SeekBar.
	KindSlider
	// KindCanvas draws shapes Go describes (Draw).
	KindCanvas
)

// Commit is one render: apply its ops in order, in one UI-thread pass.
//
//wire:message
type Commit struct {
	Seq  uint64
	Root uint32 // the root view's tag, when it changes; 0 otherwise
	Ops  []Op

	// Where the time went in Go, for diagnostics.
	BuildNs  uint64 // components and reconciliation
	LayoutNs uint64 // flexbox, including text measurement
	Measures uint32 // text measurements that reached the host
}

// Op changes one native view.
//
//wire:union
type Op interface{ isOp() }

// Create makes a view for Tag. It is not shown until a Children op places it.
//
//wire:variant Op
type Create struct {
	Tag  uint32
	Kind Kind
}

// Destroy forgets views (and removes them from their parents).
//
//wire:variant Op
type Destroy struct{ Tags []uint32 }

// Children sets a view's child views, in order. Views of Tag not listed are
// removed. For a scroll view the children go in its content.
//
//wire:variant Op
type Children struct {
	Tag      uint32
	Children []uint32
}

// Frame places a view relative to its parent (scroll views: their content).
//
//wire:variant Op
type Frame struct {
	Tag        uint32
	X, Y, W, H float32
}

// Box is the visual style every view kind shares.
//
//wire:variant Op
type Box struct {
	Tag         uint32
	Background  uint32
	Radius      float32
	BorderWidth float32
	BorderColor uint32
	Opacity     float32 // 0..1
	Clip        bool
	Label       string // accessibility label
	TestID      string // accessibilityIdentifier / view tag, for tests
	Href        string // where a pressable leads (the web host makes it a link)
	// Transition animates changes to this view's frame, opacity and
	// background over this many milliseconds; 0 changes them at once.
	Transition uint32
	LongPress  bool // a pressable that reports long presses

	// Blur shows what is behind the view, blurred; Background tints it.
	Blur Blur
	// A gradient painted over Background: colors at stops (0..1; evenly
	// spaced when there are none), from (GradientX0, GradientY0) to
	// (GradientX1, GradientY1) in the view's unit square. A radial one
	// spreads from the first point out to the distance to the second.
	GradientColors                                 []uint32
	GradientStops                                  []float32
	GradientRadial                                 bool
	GradientX0, GradientY0, GradientX1, GradientY1 float32
	// A shadow under the view: its color (0 for none), blur radius and offset.
	ShadowColor                    uint32
	ShadowRadius, ShadowX, ShadowY float32
	// Corners are the corners Radius rounds: a mask of Corner*; 0 is all four.
	Corners uint8
	// Hidden takes the view off the screen (Style.Hidden): not drawn, not tapped.
	Hidden bool
	// Scale (1 when 0) and Rotate (degrees) transform the view about its
	// centre, after layout: its place among its siblings doesn't change.
	Scale, Rotate float32
	// Pan is the drags the view reports (PanAny, PanX, PanY; 0 for none):
	// a one-way drag leaves the other way to whatever scrolls around it.
	Pan uint8
	// Pinch makes the view report two-finger pinches.
	Pinch bool
}

// Which drags a view reports (Box.Pan).
const (
	PanAny uint8 = iota + 1
	PanX
	PanY
)

// The phases of a gesture (Panned, Pinched).
const (
	GestureBegan uint8 = iota
	GestureChanged
	GestureEnded
	GestureCancelled
)

// Blur is how strongly what is behind a view is blurred: the platform's
// materials on iOS, a blur radius elsewhere.
//
//wire:enum
type Blur uint8

const (
	BlurNone Blur = iota
	BlurUltraThin
	BlurThin
	BlurRegular
	BlurThick
	// BlurGlass is Liquid Glass on iOS 26+ (the tint is left out: glass
	// takes its color from behind); elsewhere, BlurUltraThin with the tint.
	BlurGlass
)

// The corners in Box.Corners.
const (
	CornerTopLeft     = 1
	CornerTopRight    = 2
	CornerBottomRight = 4
	CornerBottomLeft  = 8
)

// Align is horizontal text alignment.
//
//wire:enum
type Align uint8

const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
)

// Font is shared by text, inputs and measurement.
type Font struct {
	Size   float32
	Weight uint16 // 100..900; 0 means 400
	Family string // a font the app bundles (nimbus native font add); "" is the system font
	// Tracking is extra space between letters, in points (negative tightens).
	Tracking float32
	// LineHeight is the distance between lines, in points; 0 is the font's own.
	LineHeight float32
	// Tabular gives every digit the same width, so figures line up and
	// don't jiggle as they change.
	Tabular bool
}

// Text sets a text view's content and style.
//
//wire:variant Op
type Text struct {
	Tag   uint32
	Text  string
	Font  Font
	Color uint32
	Align Align
	Lines uint32 // 0 = unlimited; truncated with an ellipsis
	// MinScale lets a one-line text shrink to fit its width, down to this
	// share of its size (0.6: 60%); 0 truncates instead.
	MinScale float32
	// Spans, when there are any, are the text in runs of their own look
	// (Text is then all of them, joined).
	Spans []Span
}

// Span is a run of a text with its own font and color.
type Span struct {
	Text  string
	Font  Font
	Color uint32
}

// Fit is how an image fills its frame.
//
//wire:enum
type Fit uint8

const (
	FitCover Fit = iota
	FitContain
	FitStretch
)

//wire:variant Op
type Image struct {
	Tag uint32
	URL string
	Fit Fit
}

// Keyboard is the soft keyboard an input asks for.
//
//wire:enum
type Keyboard uint8

const (
	KeyboardText Keyboard = iota
	KeyboardNumber
	KeyboardEmail
	KeyboardURL
)

// Input sets a text field. The host replaces the field's text only when it
// differs, so typing is never interrupted.
//
//wire:variant Op
type Input struct {
	Tag         uint32
	Value       string
	Placeholder string
	Font        Font
	Color       uint32
	Secure      bool
	Keyboard    Keyboard
	Multiline   bool // wraps and grows with Return making new lines
	// Submit makes Return send (Submitted) instead of a new line; the key
	// says Send.
	Submit bool
	// AutoFocus focuses the input when it appears, keyboard up.
	AutoFocus bool
}

// Screen sets a stack screen's title. Tag is the screen's root view, a
// child of a KindStack view.
//
//wire:variant Op
type Screen struct {
	Tag   uint32
	Title string
}

// TabItem sets a tab's title and icon (an SF Symbol name on iOS). Tag is
// the tab's root view, a child of a KindTabs view.
//
//wire:variant Op
type TabItem struct {
	Tag   uint32
	Title string
	Icon  string
}

// TabSelect shows the tab at Index.
//
//wire:variant Op
type TabSelect struct {
	Tag   uint32
	Index uint32
}

// Chrome is the theme for what the host draws itself: navigation and tab
// bars, screen and modal backgrounds, the status bar. Sent when it changes.
//
//wire:variant Op
type Chrome struct {
	Dark       bool
	Background uint32 // screens, modals, the root
	Text       uint32
	TextMuted  uint32 // unselected tabs
	Accent     uint32 // back buttons, selected tabs
	Bar        uint32 // navigation and tab bar backgrounds
	BarText    uint32 // titles on bars
	Border     uint32 // bar separators
}

// Present shows a KindModal view over the app, with the platform's
// transition.
//
//wire:variant Op
type Present struct{ Tag uint32 }

// Scroll sets a scroll view's content size. With Report, the host sends
// Scrolled whenever the view scrolls (lists need it to know what to show).
//
//wire:variant Op
type Scroll struct {
	Tag        uint32
	W, H       float32
	Horizontal bool
	Report     bool
	// Refreshable shows pull to refresh; pulling sends Refresh. Refreshing
	// keeps the indicator showing until Go clears it.
	Refreshable bool
	Refreshing  bool
	// StickToEnd keeps the end in view as the content grows (a chat): when
	// it gets longer, the view scrolls to show its end.
	StickToEnd bool
}

// Draw sets what a canvas draws: commands in a W×H space fitted to the
// view (centred, keeping its aspect, like an SVG viewBox). Each command takes
// its numbers from Nums and its colors from Colors, in order:
//
//	DrawMoveTo x y · DrawLineTo x y · DrawQuadTo cx cy x y
//	DrawCubicTo c1x c1y c2x c2y x y · DrawClose
//	DrawCircle cx cy r · DrawRect x y w h radius   (add to the path)
//	DrawFill color · DrawStroke width color        (paint the path, then start a new one)
//	DrawFillGradient radial x0 y0 x1 y1 n stop×n colors×n
//	                                               (fill with a gradient from (x0, y0) to (x1, y1);
//	                                               radial: from (x0, y0) out to the distance to (x1, y1))
//
//wire:variant Op
type Draw struct {
	Tag    uint32
	W, H   float32
	Cmds   []uint8
	Nums   []float32
	Colors []uint32
}

// The commands in Draw.Cmds.
const (
	DrawMoveTo uint8 = iota
	DrawLineTo
	DrawQuadTo
	DrawCubicTo
	DrawClose
	DrawCircle
	DrawRect
	DrawFill
	DrawStroke
	DrawFillGradient
)

// Slider sets a slider.
//
//wire:variant Op
type Slider struct {
	Tag      uint32
	Value    float32
	Min, Max float32
	Enabled  bool
	Tint     uint32
}

// Switch sets a toggle's state.
//
//wire:variant Op
type Switch struct {
	Tag     uint32
	On      bool
	Enabled bool
	Tint    uint32 // the on color; 0 is the platform's
}

//wire:variant Op
type Spinner struct {
	Tag   uint32
	Color uint32
}

// Viewport is the space the root fills; hosts send it at start and whenever
// it changes.
//
//wire:message
type Viewport struct {
	W, H                float32
	Scale               float32 // pixels per point / dp
	SafeTop, SafeBottom float32
	SafeLeft, SafeRight float32
	Platform            string // "ios" | "android"
	Dark                bool
	// Keyboard is how far the on-screen keyboard covers the root view from
	// its bottom edge; 0 when hidden or when the window already shrank.
	Keyboard float32
}

// Press reports a tap on a pressable.
//
//wire:message
type Press struct{ Tag uint32 }

// Scrolled reports a scroll view's offset and visible size (only for views
// whose Scroll op asked for it).
//
//wire:message
type Scrolled struct {
	Tag  uint32
	X, Y float32
	W, H float32
}

// Popped reports that the user went back in a stack (back button, swipe or
// system back), leaving Depth screens.
//
//wire:message
type Popped struct {
	Tag   uint32
	Depth uint32
}

// Dismissed reports that the user closed a modal (swipe down, back button).
//
//wire:message
type Dismissed struct{ Tag uint32 }

// Selected reports that the user picked the tab at Index.
//
//wire:message
type Selected struct {
	Tag   uint32
	Index uint32
}

// Resized reports the size a stack or tab bar gives its screens. Bottom is
// how much of that height a bar covers (iOS tabs run under the tab bar);
// content should keep clear of it.
//
//wire:message
type Resized struct {
	Tag    uint32
	W, H   float32
	Bottom float32
}

// Submitted reports Return on an input that submits (Input.Submit).
//
//wire:message
type Submitted struct {
	Tag uint32
}

// Changed reports an input's new text.
//
//wire:message
type Changed struct {
	Tag  uint32
	Text string
}

// MeasureText asks the host for a text's size (synchronous host call).
//
//wire:message
type MeasureText struct {
	Text     string
	Font     Font
	MaxWidth float32 // < 0: no limit
	Lines    uint32
	Spans    []Span // a rich text's runs (Text is them joined)
}

// TextSize answers MeasureText.
//
//wire:message
type TextSize struct{ W, H float32 }

// PluginCall asks a native plugin to run a method (synchronous host call).
// The host answers at once with PluginAck and sends the outcome later as a
// PluginResult with the same ID. Args and results are JSON.
//
//wire:message
type PluginCall struct {
	ID     uint64
	Plugin string
	Method string
	Args   string
}

// PluginAck answers PluginCall: Err is empty when the call started.
//
//wire:message
type PluginAck struct{ Err string }

// PluginResult is a plugin call's outcome. A failure has a Code
// ("unavailable", "cancelled", "denied", "not_found", "failed") and Err.
//
//wire:message
type PluginResult struct {
	ID    uint64
	Value string
	Code  string
	Err   string
}

// Toggled reports that the user flipped a switch.
//
//wire:message
type Toggled struct {
	Tag uint32
	On  bool
}

// Refresh reports that the user pulled a refreshable scroll view.
//
//wire:message
type Refresh struct{ Tag uint32 }

// Reattach reports a new native view hierarchy for a runtime that is
// already running (an Android activity created again): Go sends every
// view again, and the UI carries on with its state.
//
//wire:message
type Reattach struct{ Reason string }

// Open reports a link that opened the app (myapp://orders/42, or a web link
// the app handles), at launch or while it runs.
//
//wire:message
type Open struct{ URL string }

// Slid reports the value the user dragged a slider to.
//
//wire:message
type Slid struct {
	Tag   uint32
	Value float32
}

// LongPressed reports a long press on a pressable that asked for them.
//
//wire:message
type LongPressed struct{ Tag uint32 }

// Panned reports a drag on a view that asked for them: how far it has come
// from where it began (X, Y) and how fast it's going (VX, VY, per second),
// in points. A view that moves with the finger doesn't change these.
//
//wire:message
type Panned struct {
	Tag          uint32
	Phase        uint8
	X, Y, VX, VY float32
}

// Pinched reports a two-finger pinch: Scale is the fingers' distance over
// what it was at the start.
//
//wire:message
type Pinched struct {
	Tag   uint32
	Phase uint8
	Scale float32
}
