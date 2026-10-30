package ui

// Variant is how much a button stands out.
type Variant uint8

const (
	Filled      Variant = iota // the main action: solid Primary
	Tonal                      // a tint of Primary
	Outlined                   // a border
	Plain                      // text alone
	Destructive                // solid Danger, for what can't be undone
)

// Size is how large a control is.
type Size uint8

const (
	SizeMedium Size = iota
	SizeSmall
	SizeLarge
)

// fade gives a color an alpha, 0..1.
func fade(c Color, alpha float32) Color { return c&^0xff | Color(uint8(alpha*255)) }

// colors are a variant's background, foreground and border.
func (v Variant) colors(th Theme) (bg, fg, border Color) {
	switch v {
	case Tonal:
		return fade(th.Primary, 0.12), th.Primary, 0
	case Outlined:
		return 0, th.Text, th.Border
	case Plain:
		return 0, th.Primary, 0
	case Destructive:
		return th.Danger, th.OnPrimary, 0
	}
	return th.Primary, th.OnPrimary, 0
}

// Button is a tappable action:
//
//	ui.C(ui.Button{Label: "Place order", Icon: "check", OnPress: place})
//	ui.C(ui.Button{Label: "Cancel", Variant: ui.Outlined, OnPress: cancel})
//
// While Loading it shows a spinner and can't be tapped.
type Button struct {
	Label    string
	Icon     string // an Icon name, before the label
	OnPress  func()
	Variant  Variant
	Size     Size
	Disabled bool
	Loading  bool
	Full     bool // as wide as its container
}

func (b Button) Render(c *Ctx) *Element {
	th := UseTheme(c)
	bg, fg, border := b.Variant.colors(th)
	height, pad, font, icon := float32(46), float32(18), float32(15), float32(19)
	switch b.Size {
	case SizeSmall:
		height, pad, font, icon = 36, 14, 13.5, 16
	case SizeLarge:
		height, pad, font, icon = 54, 24, 17, 21
	}
	s := Style{Height: Pt(height), Padding: XY(pad, 0), Radius: th.Radius, Background: bg, Direction: Row,
		AlignItems: AlignCenter, Justify: JustifyCenter, Gap: 8, Transition: 150}
	if border != 0 {
		s.BorderWidth, s.BorderColor = 1, border
	}
	if b.Full {
		s.AlignSelf = AlignStretch
	} else {
		s.AlignSelf = AlignStart
	}
	onPress := b.OnPress
	if b.Disabled || b.Loading {
		onPress = nil
	}
	if b.Disabled {
		s.Opacity = 0.45
	}
	var kids []Component
	switch {
	case b.Loading:
		kids = append(kids, Spinner(Style{Color: fg, Width: Pt(icon), Height: Pt(icon)}))
	case b.Icon != "":
		kids = append(kids, Icon(b.Icon, icon, fg))
	}
	if b.Label != "" {
		kids = append(kids, Text(b.Label, Style{Color: fg, FontSize: font, FontWeight: 600}))
	}
	return Pressable(onPress, s, kids...).Label(b.Label)
}

// IconButton is a button that is only an icon. Label says what it does, for
// screen readers.
type IconButton struct {
	Icon    string
	Label   string
	OnPress func()
	Variant Variant // Plain by default
	Size    Size
	Filled  bool // solid Primary, not the plain icon
}

func (b IconButton) Render(c *Ctx) *Element {
	th := UseTheme(c)
	bg, fg, border := Color(0), th.Text, Color(0)
	if b.Filled {
		bg, fg = th.Primary, th.OnPrimary
	} else if b.Variant != Filled {
		bg, fg, border = b.Variant.colors(th)
	}
	box, icon := float32(44), float32(22)
	switch b.Size {
	case SizeSmall:
		box, icon = 34, 18
	case SizeLarge:
		box, icon = 54, 26
	}
	s := Style{Width: Pt(box), Height: Pt(box), Radius: box / 2, Background: bg, Justify: JustifyCenter, AlignItems: AlignCenter}
	if border != 0 {
		s.BorderWidth, s.BorderColor = 1, border
	}
	return Pressable(b.OnPress, s, Icon(b.Icon, icon, fg)).Label(b.Label)
}

// FloatingButton is a screen's main action, floating over its content at
// the bottom right. Put it last among the screen's children.
type FloatingButton struct {
	Icon     string
	Label    string // shown beside the icon when Extended, always read aloud
	OnPress  func()
	Extended bool
}

func (b FloatingButton) Render(c *Ctx) *Element {
	th := UseTheme(c)
	vp := Viewport(c)
	s := Style{Absolute: true, Right: Pt(20), Bottom: Pt(20 + vp.SafeBottom), Height: Pt(58), Radius: 29, Background: th.Primary,
		Direction: Row, AlignItems: AlignCenter, Justify: JustifyCenter, Gap: 10, Transition: 200,
		Shadow: &Shadow{Color: fade(th.Primary, 0.4), Radius: 18, Y: 8}}
	kids := []Component{Icon(b.Icon, 24, th.OnPrimary)}
	if b.Extended {
		s.Padding = XY(22, 0)
		kids = append(kids, Text(b.Label, Style{Color: th.OnPrimary, FontSize: 15, FontWeight: 600}))
	} else {
		s.Width = Pt(58)
	}
	return Pressable(b.OnPress, s, kids...).Label(b.Label)
}
