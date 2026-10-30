package ui

import "github.com/CodeSyncr/nimbus-native/ui/render"

// Theme is an app's design tokens. The runtime picks the light or dark one
// to match the system, uses it for anything left unstyled (text color,
// spinners, screen backgrounds) and for what the platform draws (bars,
// modal backgrounds, the status bar). Components read it with UseTheme.
type Theme struct {
	Dark bool

	Background Color // screens
	Surface    Color // cards, rows, inputs
	Text       Color
	TextMuted  Color
	Border     Color
	Primary    Color // buttons, links, selected tabs
	OnPrimary  Color // text on Primary
	Danger     Color // errors
	Success    Color // done, paid, online
	Warning    Color // needs attention
	Bar        Color // navigation and tab bars
	BarText    Color // titles on bars

	Radius float32 // corner radius for cards and buttons

	// Type scale, in points.
	Title, Heading, Body, Caption float32

	// Spacing scale, in points.
	SpaceS, SpaceM, SpaceL float32
}

// LightTheme and DarkTheme are the built-in themes.
var (
	LightTheme = Theme{
		Background: Hex("#f9fafb"), Surface: Hex("#ffffff"), Text: Hex("#111827"), TextMuted: Hex("#6b7280"),
		Border: Hex("#e5e7eb"), Primary: Hex("#2563eb"), OnPrimary: Hex("#ffffff"), Danger: Hex("#dc2626"),
		Bar: Hex("#ffffff"), BarText: Hex("#111827"), Success: Hex("#16a34a"), Warning: Hex("#d97706"),
		Radius: 12, Title: 24, Heading: 17, Body: 15, Caption: 13, SpaceS: 8, SpaceM: 12, SpaceL: 20,
	}
	DarkTheme = Theme{
		Dark:       true,
		Background: Hex("#0b0f17"), Surface: Hex("#151b26"), Text: Hex("#f3f4f6"), TextMuted: Hex("#9ca3af"),
		Border: Hex("#273041"), Primary: Hex("#3b82f6"), OnPrimary: Hex("#ffffff"), Danger: Hex("#f87171"),
		Bar: Hex("#151b26"), BarText: Hex("#f3f4f6"), Success: Hex("#4ade80"), Warning: Hex("#fbbf24"),
		Radius: 12, Title: 24, Heading: 17, Body: 15, Caption: 13, SpaceS: 8, SpaceM: 12, SpaceL: 20,
	}
)

var themes = [2]Theme{LightTheme, DarkTheme}

// SetTheme replaces the light and dark themes. Call it from init, next to
// Mount.
func SetTheme(light, dark Theme) {
	light.Dark, dark.Dark = false, true
	themes = [2]Theme{light, dark}
}

// UseTheme is the theme in effect: the dark one while the system is in dark
// mode. Components re-render when it switches.
func UseTheme(c *Ctx) Theme { return c.rt.theme() }

func (rt *runtime) theme() Theme {
	if rt.viewport.Dark {
		return themes[1]
	}
	return themes[0]
}

// chrome sends the theme to the host when it changed.
func (rt *runtime) chrome() {
	t := rt.theme()
	c := render.Chrome{Dark: t.Dark, Background: uint32(t.Background), Text: uint32(t.Text), TextMuted: uint32(t.TextMuted),
		Accent: uint32(t.Primary), Bar: uint32(t.Bar), BarText: uint32(t.BarText), Border: uint32(t.Border)}
	if c != rt.sentChrome {
		rt.sentChrome = c
		rt.op(&c)
	}
}
