// Package symbols draws the platforms' own icon sets: SF Symbols on iOS,
// and Google's Material Symbols on Android. Each icon is named per platform,
// since the two sets name things differently.
//
//	nimbus native plugin add symbols
//	nimbus native font add -name "Material Symbols Rounded" MaterialSymbolsRounded.ttf
//
//	symbols.Symbol{IOS: "cup.and.saucer.fill", Android: "local_cafe", Size: 28, Label: "Coffee"}
//
// iOS has SF Symbols built in. Android has no named icon set, so it draws
// Material Symbols from the font, which you download from Google Fonts
// (github.com/google/material-design-icons, under variablefont/) and add
// as above. The web draws the Android name with the same font, which the
// site serves from the fonts folder. For the same icon everywhere, ui.Icon
// is built in.
package symbols

import (
	"strings"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// AndroidFont is the family name the Material Symbols font was added
// under. Use "Material Symbols Outlined" or "Material Symbols Sharp" for
// those styles.
var AndroidFont = "Material Symbols Rounded"

// Symbol is one icon.
type Symbol struct {
	IOS     string   // an SF Symbol's name, such as "heart.fill"; empty draws nothing on iOS
	Android string   // a Material Symbol's name, such as "favorite"; empty draws nothing on Android
	Size    float32  // points; 0 means 24
	Color   ui.Color // the theme's text color when 0
	Weight  uint16   // 100..900, like FontWeight; 0 means 400
	// Label is what screen readers say. On iOS, empty leaves the icon out
	// for them, as for one beside text that says the same. On Android the
	// icon is text, so empty says its name: "local cafe".
	Label string
}

type props struct {
	Name   string  `json:"name"`
	Size   float32 `json:"size"`
	Color  uint32  `json:"color"`
	Weight uint16  `json:"weight"`
}

func (s Symbol) defaults(th ui.Theme) Symbol {
	if s.Size <= 0 {
		s.Size = 24
	}
	if s.Color == 0 {
		s.Color = th.Text
	}
	if s.Weight == 0 {
		s.Weight = 400
	}
	return s
}

// Render draws the icon, size by size points.
func (s Symbol) Render(c *ui.Ctx) ui.Component {
	s = s.defaults(ui.UseTheme(c))
	box := ui.Style{Width: ui.Pt(s.Size), Height: ui.Pt(s.Size)}
	var e *ui.Element
	switch {
	case (ui.Viewport(c).Platform == "android" || ui.Viewport(c).Platform == "web") && s.Android != "":
		// The font turns the name into the icon: a ligature.
		// A screen reader would read the name out, so it gets a label either way.
		spoken := s.Label
		if spoken == "" {
			spoken = strings.ReplaceAll(s.Android, "_", " ")
		}
		return ui.View(ui.Style{Width: box.Width, Height: box.Height, AlignItems: ui.AlignCenter, Justify: ui.JustifyCenter},
			ui.Text(s.Android, ui.Style{FontFamily: AndroidFont, FontSize: s.Size, FontWeight: s.Weight, Color: s.Color, LineHeight: s.Size, Lines: 1}).Label(spoken))
	case ui.Viewport(c).Platform == "ios" && s.IOS != "":
		e = ui.Native("symbol", props{s.IOS, s.Size, uint32(s.Color), s.Weight}, box)
	default:
		e = ui.View(box)
	}
	if s.Label != "" {
		e = e.Label(s.Label)
	}
	return e
}
