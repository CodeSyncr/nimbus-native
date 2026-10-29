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
}

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
}

// TextSize answers MeasureText.
//
//wire:message
type TextSize struct{ W, H float32 }
