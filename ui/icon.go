package ui

import (
	"strconv"
	"strings"
)

// Icon draws one of the built-in line icons, the same on every platform, in
// color (the theme's text color when 0):
//
//	ui.Icon("search", 22, th.TextMuted)
//
// IconNames lists them. An unknown name draws nothing. For an icon of your
// own, draw it with Canvas.
func Icon(name string, size float32, color Color) *Element {
	return C(iconView{name, size, color})
}

type iconView struct {
	name  string
	size  float32
	color Color
}

func (i iconView) Render(c *Ctx) *Element {
	color := i.color
	if color == 0 {
		color = UseTheme(c).Text
	}
	path := iconPaths[i.name]
	return Canvas(24, 24, func(p *Painter) { paintIcon(p, path, color) },
		Style{Width: Pt(i.size), Height: Pt(i.size)})
}

// iconPaths are the icons on a 24×24 grid, as paths: M move, L line, Q and C
// curves, Z close, O a circle (x y r), R a rounded rectangle (x y w h r).
var iconPaths = map[string]string{
	"plus":          "M12 5 L12 19 M5 12 L19 12",
	"minus":         "M5 12 L19 12",
	"close":         "M6 6 L18 18 M18 6 L6 18",
	"check":         "M5 12.5 L10 17.5 L19 7",
	"chevron-left":  "M15 5 L8 12 L15 19",
	"chevron-right": "M9 5 L16 12 L9 19",
	"chevron-up":    "M5 15 L12 8 L19 15",
	"chevron-down":  "M5 9 L12 16 L19 9",
	"arrow-left":    "M20 12 L4 12 M10 6 L4 12 L10 18",
	"arrow-right":   "M4 12 L20 12 M14 6 L20 12 L14 18",
	"arrow-up":      "M12 20 L12 4 M6 10 L12 4 L18 10",
	"arrow-down":    "M12 4 L12 20 M6 14 L12 20 L18 14",
	"menu":          "M4 7 L20 7 M4 12 L20 12 M4 17 L20 17",
	"more":          "O5.5 12 1.2 O12 12 1.2 O18.5 12 1.2",
	"search":        "O11 11 6.5 M16 16 L20.5 20.5",
	"filter":        "M4 6 L20 6 M7 12 L17 12 M10 18 L14 18",
	"home":          "M4 11 L12 4 L20 11 M6 9.5 L6 20 L18 20 L18 9.5 M10 20 L10 14 L14 14 L14 20",
	"settings":      "O12 12 3 M12 3 L12 6 M12 18 L12 21 M3 12 L6 12 M18 12 L21 12 M5.6 5.6 L7.8 7.8 M16.2 16.2 L18.4 18.4 M5.6 18.4 L7.8 16.2 M16.2 7.8 L18.4 5.6",
	"user":          "O12 8 4 M4.5 20.5 C5.5 16 9 14.5 12 14.5 C15 14.5 18.5 16 19.5 20.5",
	"users":         "O9 8.5 3.5 M3 20 C3.8 16 6.5 14.5 9 14.5 C11.5 14.5 14.2 16 15 20 M15.5 5.2 C17.6 5.4 18.8 7 18.8 8.6 C18.8 10.2 17.6 11.7 15.5 12 M17.5 14.8 C19.4 15.5 20.6 17.4 21 20",
	"heart":         "M12 20 C5 15 3.5 11.5 3.5 9 C3.5 6.5 5.5 4.5 8 4.5 C9.7 4.5 11.2 5.5 12 7 C12.8 5.5 14.3 4.5 16 4.5 C18.5 4.5 20.5 6.5 20.5 9 C20.5 11.5 19 15 12 20 Z",
	"star":          "M12 3.5 L14.6 9 L20.5 9.8 L16.2 13.9 L17.3 19.8 L12 17 L6.7 19.8 L7.8 13.9 L3.5 9.8 L9.4 9 Z",
	"bell":          "M6 16.5 L6 11 C6 7.5 8.5 5 12 5 C15.5 5 18 7.5 18 11 L18 16.5 L20 18.5 L4 18.5 Z M10 21 L14 21",
	"mail":          "R3.5 5.5 17 13 2.5 M4 7 L12 13 L20 7",
	"phone":         "M7 3.5 L9.5 3.5 L11 8 L8.8 9.8 C9.9 12.2 11.8 14.1 14.2 15.2 L16 13 L20.5 14.5 L20.5 17 C20.5 19 18.8 20.5 17 20.3 C10 19.6 4.4 14 3.7 7 C3.5 5.2 5 3.5 7 3.5 Z",
	"camera":        "R3 7 18 13 2.5 M8.5 7 L10 4.5 L14 4.5 L15.5 7 O12 13.5 3.5",
	"image":         "R3.5 4.5 17 15 2.5 O9 9.5 1.7 M4 17 L9.5 12.5 L13 15.5 L16 13 L20 16.5",
	"trash":         "M4 7 L20 7 M9 7 L9 4.5 L15 4.5 L15 7 M6.5 7 L7.5 20 L16.5 20 L17.5 7 M10.3 11 L10.3 16.5 M13.7 11 L13.7 16.5",
	"edit":          "M5 19 L5.8 15 L16 4.8 C16.8 4 18.2 4 19 4.8 C19.8 5.6 19.8 7 19 7.8 L8.8 18 Z M14 7 L17 10",
	"share":         "M12 15 L12 4 M8 8 L12 4 L16 8 M7 11 L5 11 L5 20 L19 20 L19 11 L17 11",
	"download":      "M12 4 L12 15 M7.5 10.5 L12 15 L16.5 10.5 M4.5 19.5 L19.5 19.5",
	"upload":        "M12 15 L12 4 M7.5 8.5 L12 4 L16.5 8.5 M4.5 19.5 L19.5 19.5",
	"calendar":      "R4 5.5 16 14.5 2.5 M4 10 L20 10 M8.5 3.5 L8.5 7 M15.5 3.5 L15.5 7",
	"clock":         "O12 12 8.5 M12 7 L12 12 L15.5 14",
	"map-pin":       "M12 21 C7 15.5 5 12.5 5 9.5 C5 5.8 8.2 3 12 3 C15.8 3 19 5.8 19 9.5 C19 12.5 17 15.5 12 21 Z O12 9.5 2.5",
	"lock":          "R5 10.5 14 10 2.5 M8 10.5 L8 7.5 C8 5.2 9.8 3.5 12 3.5 C14.2 3.5 16 5.2 16 7.5 L16 10.5",
	"eye":           "M2.5 12 C5 7.5 8.3 5.5 12 5.5 C15.7 5.5 19 7.5 21.5 12 C19 16.5 15.7 18.5 12 18.5 C8.3 18.5 5 16.5 2.5 12 Z O12 12 3",
	"eye-off":       "M4 4 L20 20 M9.5 5.9 C10.3 5.6 11.1 5.5 12 5.5 C15.7 5.5 19 7.5 21.5 12 C20.7 13.4 19.8 14.6 18.8 15.5 M15 16.9 C14 17.9 13 18.5 12 18.5 C8.3 18.5 5 16.5 2.5 12 C3.4 10.4 4.5 9.1 5.7 8.1",
	"info":          "O12 12 8.5 M12 11 L12 16.5 M12 7.8 L12 8",
	"alert":         "M12 4 L21.5 20 L2.5 20 Z M12 10 L12 14.5 M12 17 L12 17.2",
	"help":          "O12 12 8.5 M9.5 9.5 C9.7 8 10.8 7.3 12 7.3 C13.4 7.3 14.6 8.2 14.6 9.6 C14.6 11.6 12 11.6 12 13.7 M12 16.5 L12 16.7",
	"cart":          "M3 4.5 L5.5 4.5 L8 15.5 L18 15.5 L20 7.5 L6.2 7.5 O9 19.5 1.3 O17 19.5 1.3",
	"bag":           "M5 8 L19 8 L20 20.5 L4 20.5 Z M8.5 10.5 L8.5 7 C8.5 5 10 3.5 12 3.5 C14 3.5 15.5 5 15.5 7 L15.5 10.5",
	"card":          "R3 5.5 18 13 2.5 M3 10 L21 10 M6.5 15 L10 15",
	"file":          "M6 3.5 L14 3.5 L18.5 8 L18.5 20.5 L6 20.5 Z M14 3.5 L14 8 L18.5 8",
	"folder":        "M3.5 6.5 L9.5 6.5 L11.5 9 L20.5 9 L20.5 19 L3.5 19 Z",
	"link":          "M10 14 C11.2 15.4 13.2 15.5 14.6 14.2 L18 10.8 C19.4 9.4 19.4 7.2 18 5.9 C16.7 4.6 14.6 4.6 13.2 5.9 L12 7.1 M14 10 C12.8 8.6 10.8 8.5 9.4 9.8 L6 13.2 C4.6 14.6 4.6 16.8 6 18.1 C7.3 19.4 9.4 19.4 10.8 18.1 L12 16.9",
	"copy":          "R8.5 8.5 12 12 2 M15.5 8.5 L15.5 5.5 C15.5 4.4 14.6 3.5 13.5 3.5 L5.5 3.5 C4.4 3.5 3.5 4.4 3.5 5.5 L3.5 13.5 C3.5 14.6 4.4 15.5 5.5 15.5 L8.5 15.5",
	"refresh":       "M19.5 12 C19.5 16.2 16.2 19.5 12 19.5 C9.4 19.5 7.1 18.2 5.8 16.2 M4.5 12 C4.5 7.8 7.8 4.5 12 4.5 C14.6 4.5 16.9 5.8 18.2 7.8 M18.5 3.8 L18.5 8 L14.3 8 M5.5 20.2 L5.5 16 L9.7 16",
	"send":          "M4 12 L20.5 4 L15 20 L11.5 13 Z M11.5 13 L20.5 4",
	"mic":           "R9 3.5 6 11 3 M5.5 11.5 C5.5 15.2 8.4 17.5 12 17.5 C15.6 17.5 18.5 15.2 18.5 11.5 M12 17.5 L12 20.5",
	"play":          "M7 4.5 L19 12 L7 19.5 Z",
	"pause":         "M8.5 5 L8.5 19 M15.5 5 L15.5 19",
	"sun":           "O12 12 3.8 M12 3 L12 5 M12 19 L12 21 M3 12 L5 12 M19 12 L21 12 M5.6 5.6 L7 7 M17 17 L18.4 18.4 M5.6 18.4 L7 17 M17 7 L18.4 5.6",
	"moon":          "M19.5 14.5 C15 16 9 12 9.5 4.5 C5.8 5.8 3.8 9 3.8 12.2 C3.8 16.8 7.5 20.5 12 20.5 C15.2 20.5 18.2 18.3 19.5 14.5 Z",
	"globe":         "O12 12 8.5 M3.5 12 L20.5 12 M12 3.5 C9.5 6 8.5 9 8.5 12 C8.5 15 9.5 18 12 20.5 C14.5 18 15.5 15 15.5 12 C15.5 9 14.5 6 12 3.5 Z",
	"wifi":          "M3 9.5 C8.2 4.8 15.8 4.8 21 9.5 M6.2 13 C9.6 10 14.4 10 17.8 13 M9.4 16.4 C11 15.2 13 15.2 14.6 16.4 M12 19.5 L12 19.7",
	"tag":           "M4 4 L12 4 L20.5 12.5 L12.5 20.5 L4 12 Z O8.5 8.5 1.2",
	"gift":          "R3.5 8.5 17 4 1 M5 12.5 L5 20.5 L19 20.5 L19 12.5 M12 8.5 L12 20.5 M12 8.5 C10 4 6 3.5 6 6 C6 8 9 8.5 12 8.5 C15 8.5 18 8 18 6 C18 3.5 14 4 12 8.5",
}

// IconNames are the built-in icons.
func IconNames() []string {
	out := make([]string, 0, len(iconPaths))
	for n := range iconPaths {
		out = append(out, n)
	}
	return out
}

// paintIcon draws a path from iconPaths.
func paintIcon(p *Painter, path string, color Color) {
	f := strings.Fields(path)
	num := func(s string) float32 {
		v, _ := strconv.ParseFloat(s, 32)
		return float32(v)
	}
	args := func(i, n int) []float32 {
		out := make([]float32, n)
		out[0] = num(f[i][1:])
		for k := 1; k < n; k++ {
			out[k] = num(f[i+k])
		}
		return out
	}
	for i := 0; i < len(f); {
		switch f[i][0] {
		case 'M':
			a := args(i, 2)
			p.MoveTo(a[0], a[1])
			i += 2
		case 'L':
			a := args(i, 2)
			p.LineTo(a[0], a[1])
			i += 2
		case 'Q':
			a := args(i, 4)
			p.QuadTo(a[0], a[1], a[2], a[3])
			i += 4
		case 'C':
			a := args(i, 6)
			p.CubicTo(a[0], a[1], a[2], a[3], a[4], a[5])
			i += 6
		case 'O':
			a := args(i, 3)
			p.Circle(a[0], a[1], a[2])
			i += 3
		case 'R':
			a := args(i, 5)
			p.Rect(a[0], a[1], a[2], a[3], a[4])
			i += 5
		case 'Z':
			p.Close()
			i++
		default:
			i++
		}
	}
	p.Stroke(1.8, color)
}
