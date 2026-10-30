package web

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/CodeSyncr/nimbus-native/layout"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// The web host turns render ops into a few DOM instructions. The same
// instructions build the server-rendered HTML (dom, here) and drive the
// browser (host.js), so every mapping from the protocol to HTML and CSS
// lives in this file.
//
//	{"c":[tag,"div","v"]}          create (or replace) an element: tag name, class
//	{"k":[tag,[child tags]]}       set children, in order
//	{"s":[tag,"l"|"b"|"x",css]}    set one part of the inline style (layout, box, extra)
//	{"a":[tag,name,value|null]}    set or remove an attribute
//	{"p":[tag,name,value]}         set a DOM property (value, checked)
//	{"t":[tag,text]}               set text content
//	{"d":[tags]}                   destroy
//	{"r":tag}                      the root element
//	{"m":tag}                      present a modal
//	{"th":{...}}                   theme colors
type instr map[string]any

// translator turns ops into instructions, remembering what it needs about
// each element (its kind, whether an input is multiline).
type translator struct {
	kinds     map[uint32]render.Kind
	multiline map[uint32]bool
}

func newTranslator() *translator {
	return &translator{kinds: map[uint32]render.Kind{}, multiline: map[uint32]bool{}}
}

func (t *translator) commit(c *render.Commit, styles map[uint32]layout.Style) []instr {
	var out []instr
	for _, op := range c.Ops {
		out = append(out, t.op(op)...)
	}
	// Layout styles after the ops: the elements exist by now.
	tags := make([]uint32, 0, len(styles))
	for tag := range styles {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i] < tags[j] })
	for _, tag := range tags {
		out = append(out, instr{"s": []any{tag, "l", layoutCSS(t.kinds[tag], styles[tag])}})
	}
	if c.Root != 0 {
		out = append(out, instr{"r": c.Root})
	}
	return out
}

var elements = map[render.Kind][2]string{
	render.KindView:      {"div", "v"},
	render.KindText:      {"div", "t"},
	render.KindImage:     {"img", "i"},
	render.KindPressable: {"a", "p"},
	render.KindScroll:    {"div", "sc"},
	render.KindInput:     {"input", "in"},
	render.KindSpinner:   {"div", "sp"},
	render.KindStack:     {"div", "st"},
	render.KindTabs:      {"div", "tb"},
	render.KindModal:     {"div", "md"},
	render.KindSwitch:    {"input", "sw"},
	render.KindSlider:    {"input", "sl"},
	render.KindCanvas:    {"canvas", "cv"},
}

func (t *translator) op(op render.Op) []instr {
	switch o := op.(type) {
	case *render.Create:
		t.kinds[o.Tag] = o.Kind
		e, ok := elements[o.Kind]
		if !ok {
			e = elements[render.KindView]
		}
		out := []instr{{"c": []any{o.Tag, e[0], e[1]}}}
		if o.Kind == render.KindSwitch {
			out = append(out, instr{"a": []any{o.Tag, "type", "checkbox"}}, instr{"a": []any{o.Tag, "role", "switch"}})
		}
		if o.Kind == render.KindSlider {
			out = append(out, instr{"a": []any{o.Tag, "type", "range"}}, instr{"a": []any{o.Tag, "step", "any"}})
		}
		return out
	case *render.Destroy:
		for _, tag := range o.Tags {
			delete(t.kinds, tag)
			delete(t.multiline, tag)
		}
		return []instr{{"d": o.Tags}}
	case *render.Children:
		return []instr{{"k": []any{o.Tag, o.Children}}}
	case *render.Box:
		return t.box(o)
	case *render.Draw:
		// The commands as an attribute (it survives server rendering);
		// host.js paints any canvas that has one.
		b, _ := json.Marshal(map[string]any{"w": o.W, "h": o.H, "k": o.Cmds, "n": o.Nums, "c": colorsCSS(o.Colors)})
		return []instr{{"a": []any{o.Tag, "data-draw", string(b)}}}
	case *render.Text:
		x := fontCSS(o.Font, o.Color) + "text-align:" + [...]string{"left", "center", "right"}[o.Align%3] + ";"
		if o.Lines > 0 {
			x += fmt.Sprintf("display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:%d;overflow:hidden;", o.Lines)
		}
		return []instr{{"t": []any{o.Tag, o.Text}}, {"s": []any{o.Tag, "x", x}}}
	case *render.Image:
		fit := [...]string{"cover", "contain", "fill"}[o.Fit%3]
		return []instr{{"a": []any{o.Tag, "src", o.URL}}, {"a": []any{o.Tag, "alt", ""}}, {"s": []any{o.Tag, "x", "object-fit:" + fit + ";"}}}
	case *render.Input:
		var out []instr
		if o.Multiline != t.multiline[o.Tag] {
			t.multiline[o.Tag] = o.Multiline
			name := "input"
			if o.Multiline {
				name = "textarea"
			}
			out = append(out, instr{"c": []any{o.Tag, name, "in"}})
		}
		typ := "text"
		switch {
		case o.Secure:
			typ = "password"
		case o.Keyboard == render.KeyboardEmail:
			typ = "email"
		case o.Keyboard == render.KeyboardURL:
			typ = "url"
		}
		mode := map[render.Keyboard]string{render.KeyboardNumber: "decimal", render.KeyboardEmail: "email", render.KeyboardURL: "url"}[o.Keyboard]
		if !o.Multiline {
			out = append(out, instr{"a": []any{o.Tag, "type", typ}})
		}
		out = append(out,
			instr{"a": []any{o.Tag, "placeholder", nullable(o.Placeholder)}},
			instr{"a": []any{o.Tag, "inputmode", nullable(mode)}},
			instr{"p": []any{o.Tag, "value", o.Value}},
			instr{"s": []any{o.Tag, "x", fontCSS(o.Font, o.Color)}},
			instr{"a": []any{o.Tag, "data-submit", boolAttr(o.Submit)}},
			instr{"a": []any{o.Tag, "enterkeyhint", nullable(map[bool]string{true: "send"}[o.Submit])}},
			instr{"a": []any{o.Tag, "data-af", boolAttr(o.AutoFocus)}})
		return out
	case *render.Scroll:
		x := "overflow-y:auto;overflow-x:hidden;-webkit-overflow-scrolling:touch;"
		if o.Horizontal {
			x = "overflow-x:auto;overflow-y:hidden;"
		}
		return []instr{{"s": []any{o.Tag, "x", x}}, {"a": []any{o.Tag, "data-stick", boolAttr(o.StickToEnd)}}}
	case *render.Spinner:
		return []instr{{"s": []any{o.Tag, "x", "border-top-color:" + cssColor(o.Color) + ";"}}}
	case *render.Switch:
		out := []instr{{"p": []any{o.Tag, "checked", o.On}}, {"a": []any{o.Tag, "disabled", boolAttr(!o.Enabled)}}}
		if o.Tint != 0 {
			out = append(out, instr{"s": []any{o.Tag, "x", "accent-color:" + cssColor(o.Tint) + ";"}})
		}
		return out
	case *render.Slider:
		out := []instr{{"a": []any{o.Tag, "min", num(o.Min)}}, {"a": []any{o.Tag, "max", num(o.Max)}},
			{"p": []any{o.Tag, "value", num(o.Value)}}, {"a": []any{o.Tag, "disabled", boolAttr(!o.Enabled)}}}
		if o.Tint != 0 {
			out = append(out, instr{"s": []any{o.Tag, "x", "accent-color:" + cssColor(o.Tint) + ";"}})
		}
		return out
	case *render.Screen:
		return []instr{{"a": []any{o.Tag, "data-title", o.Title}}}
	case *render.TabItem:
		return []instr{{"a": []any{o.Tag, "data-title", o.Title}}}
	case *render.TabSelect:
		return []instr{{"a": []any{o.Tag, "data-sel", strconv.Itoa(int(o.Index))}}}
	case *render.Present:
		return []instr{{"m": o.Tag}}
	case *render.Chrome:
		return []instr{{"th": map[string]any{
			"bg": cssColor(o.Background), "fg": cssColor(o.Text), "muted": cssColor(o.TextMuted), "ac": cssColor(o.Accent),
			"bar": cssColor(o.Bar), "barfg": cssColor(o.BarText), "border": cssColor(o.Border), "dark": o.Dark,
		}}}
	}
	return nil // Frame and the rest: the browser lays out
}

func (t *translator) box(o *render.Box) []instr {
	var b strings.Builder
	if o.Background != 0 {
		b.WriteString("background:" + cssColor(o.Background) + ";")
	}
	if o.Radius > 0 {
		if o.Corners == 0 {
			fmt.Fprintf(&b, "border-radius:%spx;", num(o.Radius))
		} else {
			r := func(bit uint8) string {
				if o.Corners&bit != 0 {
					return num(o.Radius) + "px"
				}
				return "0"
			}
			fmt.Fprintf(&b, "border-radius:%s %s %s %s;", r(render.CornerTopLeft), r(render.CornerTopRight), r(render.CornerBottomRight), r(render.CornerBottomLeft))
		}
	}
	if len(o.GradientColors) >= 2 {
		b.WriteString("background-image:" + gradientCSS(o) + ";")
	}
	if o.ShadowColor != 0 {
		fmt.Fprintf(&b, "box-shadow:%spx %spx %spx %s;", num(o.ShadowX), num(o.ShadowY), num(o.ShadowRadius), cssColor(o.ShadowColor))
	}
	if px := [...]int{0, 8, 14, 22, 30, 8}[min(int(o.Blur), 5)]; px > 0 {
		fmt.Fprintf(&b, "backdrop-filter:blur(%[1]dpx) saturate(180%%);-webkit-backdrop-filter:blur(%[1]dpx) saturate(180%%);", px)
	}
	if o.BorderWidth > 0 {
		b.WriteString("border-style:solid;border-color:" + cssColor(o.BorderColor) + ";")
	}
	if o.Opacity < 1 {
		fmt.Fprintf(&b, "opacity:%s;", num(o.Opacity))
	}
	if o.Clip && t.kinds[o.Tag] != render.KindScroll {
		b.WriteString("overflow:hidden;")
	}
	if o.Transition > 0 {
		fmt.Fprintf(&b, "transition:opacity %[1]dms,background-color %[1]dms,transform %[1]dms,width %[1]dms,height %[1]dms;", o.Transition)
	}
	out := []instr{{"s": []any{o.Tag, "b", b.String()}},
		{"a": []any{o.Tag, "aria-label", nullable(o.Label)}},
		{"a": []any{o.Tag, "data-testid", nullable(o.TestID)}},
		{"a": []any{o.Tag, "data-lp", boolAttr(o.LongPress)}}}
	if t.kinds[o.Tag] == render.KindPressable {
		out = append(out, instr{"a": []any{o.Tag, "href", nullable(o.Href)}})
		if o.Href == "" {
			out = append(out, instr{"a": []any{o.Tag, "role", "button"}}, instr{"a": []any{o.Tag, "tabindex", "0"}})
		}
	}
	return out
}

// layoutCSS is a node's flexbox, as the Go layout engine would compute it.
func layoutCSS(kind render.Kind, s layout.Style) string {
	var b strings.Builder
	switch {
	case s.Hidden:
		b.WriteString("display:none;")
	case kind == render.KindText:
		// Text is a leaf; its line clamp (x) may set display.
	default:
		b.WriteString("display:flex;")
	}
	dir := "column"
	if s.Direction == layout.Row {
		dir = "row"
	}
	b.WriteString("flex-direction:" + dir + ";")
	b.WriteString("justify-content:" + [...]string{"flex-start", "center", "flex-end", "space-between", "space-around", "space-evenly"}[s.Justify%6] + ";")
	align := func(a layout.Align, auto string) string {
		return [...]string{auto, "stretch", "flex-start", "center", "flex-end"}[a%5]
	}
	b.WriteString("align-items:" + align(s.AlignItems, "stretch") + ";")
	if s.AlignSelf != layout.AlignAuto {
		b.WriteString("align-self:" + align(s.AlignSelf, "auto") + ";")
	}
	fmt.Fprintf(&b, "flex-grow:%s;flex-shrink:%s;", num(s.Grow), num(s.Shrink))
	if s.Basis.U != layout.Auto {
		b.WriteString("flex-basis:" + length(s.Basis) + ";")
	}
	for _, p := range []struct {
		name string
		v    layout.Value
	}{{"width", s.Width}, {"height", s.Height}, {"min-width", s.MinWidth}, {"min-height", s.MinHeight},
		{"max-width", s.MaxWidth}, {"max-height", s.MaxHeight}} {
		if p.v.U != layout.Auto {
			b.WriteString(p.name + ":" + length(p.v) + ";")
		}
	}
	edges := func(name string, e layout.Edges) {
		if e != (layout.Edges{}) {
			fmt.Fprintf(&b, "%s:%spx %spx %spx %spx;", name, num(e.Top), num(e.Right), num(e.Bottom), num(e.Left))
		}
	}
	edges("padding", s.Padding)
	edges("margin", s.Margin)
	edges("border-width", s.Border)
	if s.Wrap {
		b.WriteString("flex-wrap:wrap;align-content:flex-start;")
	}
	if s.Gap > 0 || (s.Wrap && s.LineGap > 0) {
		line := s.Gap
		if s.Wrap && s.LineGap > 0 {
			line = s.LineGap
		}
		if s.Direction == layout.Row { // gap is "row column": lines, then items
			fmt.Fprintf(&b, "gap:%spx %spx;", num(line), num(s.Gap))
		} else {
			fmt.Fprintf(&b, "gap:%spx %spx;", num(s.Gap), num(line))
		}
	}
	if s.Position == layout.Absolute {
		b.WriteString("position:absolute;")
		for _, p := range []struct {
			name string
			v    layout.Value
		}{{"left", s.Left}, {"top", s.Top}, {"right", s.Right}, {"bottom", s.Bottom}} {
			if p.v.U != layout.Auto {
				b.WriteString(p.name + ":" + length(p.v) + ";")
			}
		}
	}
	return b.String()
}

func fontCSS(f render.Font, color uint32) string {
	s := ""
	if f.Family != "" {
		s += "font-family:\"" + strings.ReplaceAll(f.Family, `"`, "") + "\",system-ui,sans-serif;"
	}
	if f.Size > 0 {
		s += "font-size:" + num(f.Size) + "px;"
	}
	if f.Weight > 0 {
		s += "font-weight:" + strconv.Itoa(int(f.Weight)) + ";"
	}
	if f.Tracking != 0 {
		s += "letter-spacing:" + num(f.Tracking) + "px;"
	}
	if f.LineHeight > 0 {
		s += "line-height:" + num(f.LineHeight) + "px;"
	}
	if f.Tabular {
		s += "font-variant-numeric:tabular-nums;"
	}
	if color != 0 {
		s += "color:" + cssColor(color) + ";"
	}
	return s
}

func length(v layout.Value) string {
	if v.U == layout.Percent {
		return num(v.V) + "%"
	}
	return num(v.V) + "px"
}

func num(f float32) string { return strconv.FormatFloat(float64(f), 'f', -1, 32) }

// cssColor turns 0xRRGGBBAA into a CSS color.
// colorsCSS is a list of colors as CSS.
func colorsCSS(cs []uint32) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = cssColor(c)
	}
	return out
}

// gradientCSS is a Box's gradient as a CSS image: the same points, in the
// box's unit square, as the native hosts draw.
func gradientCSS(o *render.Box) string {
	var stops []string
	for i, c := range o.GradientColors {
		st := cssColor(c)
		if len(o.GradientStops) == len(o.GradientColors) {
			st += " " + num(o.GradientStops[i]*100) + "%"
		}
		stops = append(stops, st)
	}
	if o.GradientRadial {
		r := float32(math.Hypot(float64(o.GradientX1-o.GradientX0), float64(o.GradientY1-o.GradientY0))) * 100
		return fmt.Sprintf("radial-gradient(%[1]s%% %[1]s%% at %[2]s%% %[3]s%%,%[4]s)", num(r), num(o.GradientX0*100), num(o.GradientY0*100), strings.Join(stops, ","))
	}
	deg := math.Atan2(float64(o.GradientX1-o.GradientX0), float64(o.GradientY0-o.GradientY1)) * 180 / math.Pi
	return fmt.Sprintf("linear-gradient(%sdeg,%s)", num(float32(deg)), strings.Join(stops, ","))
}

func cssColor(c uint32) string {
	if c == 0 {
		return "transparent"
	}
	if c&0xff == 0xff {
		return fmt.Sprintf("#%06x", c>>8)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", c>>24, c>>16&0xff, c>>8&0xff, num(float32(c&0xff)/255))
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolAttr(b bool) any {
	if b {
		return ""
	}
	return nil
}

// dom is the page as the server renders it: the instructions applied to a
// tree of nodes, written out as HTML.
type dom struct {
	nodes  map[uint32]*node
	root   uint32
	modals []uint32
	theme  map[string]any
}

type node struct {
	name, class string
	style       map[string]string // l, b, x
	attrs       map[string]string
	props       map[string]any
	text        string
	children    []uint32
}

func newDOM() *dom { return &dom{nodes: map[uint32]*node{}} }

func (d *dom) apply(in []instr) {
	for _, i := range in {
		for k, v := range i {
			switch k {
			case "c":
				a := v.([]any)
				tag := a[0].(uint32)
				n := &node{name: a[1].(string), class: a[2].(string), style: map[string]string{}, attrs: map[string]string{}, props: map[string]any{}}
				if old := d.nodes[tag]; old != nil { // replaced: keep its styles and attributes
					n.style, n.attrs, n.props = old.style, old.attrs, old.props
				}
				d.nodes[tag] = n
			case "k":
				a := v.([]any)
				if n := d.nodes[a[0].(uint32)]; n != nil {
					n.children = append([]uint32(nil), a[1].([]uint32)...)
				}
			case "s":
				a := v.([]any)
				if n := d.nodes[a[0].(uint32)]; n != nil {
					n.style[a[1].(string)] = a[2].(string)
				}
			case "a":
				a := v.([]any)
				if n := d.nodes[a[0].(uint32)]; n != nil {
					if a[2] == nil {
						delete(n.attrs, a[1].(string))
					} else {
						n.attrs[a[1].(string)] = a[2].(string)
					}
				}
			case "p":
				a := v.([]any)
				if n := d.nodes[a[0].(uint32)]; n != nil {
					n.props[a[1].(string)] = a[2]
				}
			case "t":
				a := v.([]any)
				if n := d.nodes[a[0].(uint32)]; n != nil {
					n.text = a[1].(string)
				}
			case "d":
				for _, tag := range v.([]uint32) {
					delete(d.nodes, tag)
				}
			case "r":
				d.root = v.(uint32)
			case "m":
				d.modals = append(d.modals, v.(uint32))
			case "th":
				d.theme = v.(map[string]any)
			}
		}
	}
}

// html writes the tree under the root, then presented modals.
func (d *dom) html(b *strings.Builder) {
	d.write(b, d.root)
	for _, m := range d.modals {
		if d.nodes[m] != nil {
			b.WriteString(`<div class="ov" data-for="` + strconv.Itoa(int(m)) + `">`)
			d.write(b, m)
			b.WriteString(`</div>`)
		}
	}
}

func (d *dom) write(b *strings.Builder, tag uint32) {
	n := d.nodes[tag]
	if n == nil {
		return
	}
	fmt.Fprintf(b, `<%s class="%s" data-t="%d"`, n.name, n.class, tag)
	if css := n.style["l"] + n.style["b"] + n.style["x"]; css != "" {
		b.WriteString(` style="` + html.EscapeString(css) + `"`)
	}
	names := make([]string, 0, len(n.attrs))
	for k := range n.attrs {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(b, ` %s="%s"`, k, html.EscapeString(n.attrs[k]))
	}
	if v, ok := n.props["checked"].(bool); ok && v {
		b.WriteString(" checked")
	}
	value, _ := n.props["value"].(string)
	switch n.name {
	case "img":
		b.WriteString(">")
		return
	case "input":
		if value != "" {
			b.WriteString(` value="` + html.EscapeString(value) + `"`)
		}
		b.WriteString(">")
		return
	case "textarea":
		b.WriteString(">" + html.EscapeString(value) + "</textarea>")
		return
	}
	b.WriteString(">")
	b.WriteString(html.EscapeString(n.text))
	for _, c := range n.children {
		d.write(b, c)
	}
	b.WriteString("</" + n.name + ">")
}

// themeCSS is the theme as CSS variables for the page's first paint.
func (d *dom) themeCSS() string {
	if d.theme == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(":root{")
	for _, k := range []string{"bg", "fg", "muted", "ac", "bar", "barfg", "border"} {
		if v, ok := d.theme[k].(string); ok {
			b.WriteString("--" + k + ":" + v + ";")
		}
	}
	b.WriteString("}")
	return b.String()
}
