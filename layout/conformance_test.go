package layout

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Chromium's layout of testdata/fixtures.html is stored in
// testdata/chrome.json; -chrome regenerates it with a local Chromium browser
// (headless, throwaway profile).
var chrome = flag.String("chrome", "", "path to a Chromium-based browser binary; regenerates testdata/chrome.json")

// harnessCSS gives every div Nimbus Native's layout defaults.
const harnessCSS = `
body { margin: 0 }
section { position: relative; height: 600px }
section > div { position: absolute; left: 0; top: 0 }
div { box-sizing: border-box; display: flex; flex-direction: column; flex-shrink: 0;
      position: relative; min-width: 0; min-height: 0; border: 0 solid black; align-items: stretch;
      align-content: flex-start }
`

// tolerance covers Chromium's 1/64 px LayoutUnit rounding.
const tolerance = 0.05

type htmlNode struct {
	name     string
	style    string
	children []*htmlNode
}

var tagRE = regexp.MustCompile(`<div((?:\s+[a-z-]+="[^"]*")*)\s*>|</div>`)
var attrRE = regexp.MustCompile(`([a-z-]+)="([^"]*)"`)

func parseFixtures(t *testing.T) []*htmlNode {
	b, err := os.ReadFile("testdata/fixtures.html")
	if err != nil {
		t.Fatal(err)
	}
	src := regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(string(b), "")
	var roots, stack []*htmlNode
	for _, m := range tagRE.FindAllStringSubmatch(src, -1) {
		if m[0] == "</div>" {
			stack = stack[:len(stack)-1]
			continue
		}
		n := &htmlNode{}
		for _, a := range attrRE.FindAllStringSubmatch(m[1], -1) {
			switch a[1] {
			case "style":
				n.style = a[2]
			case "data-name":
				n.name = a[2]
			}
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			p := stack[len(stack)-1]
			p.children = append(p.children, n)
		}
		stack = append(stack, n)
	}
	return roots
}

// toLayout builds the layout tree and returns its nodes in document order.
func toLayout(t *testing.T, h *htmlNode) (*Node, []*Node) {
	var all []*Node
	var build func(h *htmlNode) *Node
	build = func(h *htmlNode) *Node {
		n := &Node{Style: parseStyle(t, h.style)}
		all = append(all, n)
		for _, c := range h.children {
			n.Children = append(n.Children, build(c))
		}
		return n
	}
	return build(h), all
}

func parseStyle(t *testing.T, css string) Style {
	var s Style
	for _, decl := range strings.Split(css, ";") {
		k, v, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		val := func(x string) Value {
			switch {
			case x == "auto":
				return Value{}
			case strings.HasSuffix(x, "%"):
				return Pct(num(t, strings.TrimSuffix(x, "%")))
			}
			return Pt(num(t, strings.TrimSuffix(x, "px")))
		}
		pt := func(x string) float32 { return num(t, strings.TrimSuffix(x, "px")) }
		edges := func(x string) Edges {
			p := strings.Fields(x)
			switch len(p) {
			case 1:
				return All(pt(p[0]))
			case 2:
				return Edges{pt(p[1]), pt(p[0]), pt(p[1]), pt(p[0])}
			case 3:
				return Edges{pt(p[1]), pt(p[0]), pt(p[1]), pt(p[2])}
			}
			return Edges{pt(p[3]), pt(p[0]), pt(p[1]), pt(p[2])}
		}
		switch k {
		case "width":
			s.Width = val(v)
		case "height":
			s.Height = val(v)
		case "min-width":
			s.MinWidth = val(v)
		case "min-height":
			s.MinHeight = val(v)
		case "max-width":
			s.MaxWidth = val(v)
		case "max-height":
			s.MaxHeight = val(v)
		case "flex-direction":
			s.Direction = map[string]Direction{"row": Row, "column": Column}[v]
		case "justify-content":
			s.Justify = map[string]Justify{"flex-start": JustifyStart, "center": JustifyCenter, "flex-end": JustifyEnd,
				"space-between": JustifySpaceBetween, "space-around": JustifySpaceAround, "space-evenly": JustifySpaceEvenly}[v]
		case "align-items", "align-self":
			a := map[string]Align{"stretch": AlignStretch, "flex-start": AlignStart, "center": AlignCenter, "flex-end": AlignEnd}[v]
			if k == "align-items" {
				s.AlignItems = a
			} else {
				s.AlignSelf = a
			}
		case "flex-grow":
			s.Grow = num(t, v)
		case "flex-shrink":
			s.Shrink = num(t, v)
		case "flex-basis":
			s.Basis = val(v)
		case "padding":
			s.Padding = edges(v)
		case "margin":
			s.Margin = edges(v)
		case "border-width":
			s.Border = edges(v)
		case "padding-left":
			s.Padding.Left = pt(v)
		case "margin-left":
			s.Margin.Left = pt(v)
		case "margin-right":
			s.Margin.Right = pt(v)
		case "margin-top":
			s.Margin.Top = pt(v)
		case "gap":
			s.Gap = pt(v)
		case "position":
			if v == "absolute" {
				s.Position = Absolute
			}
		case "left":
			s.Left = val(v)
		case "top":
			s.Top = val(v)
		case "right":
			s.Right = val(v)
		case "bottom":
			s.Bottom = val(v)
		case "flex-wrap":
			s.Wrap = v == "wrap"
		case "display":
			s.Hidden = v == "none"
		default:
			t.Fatalf("fixture uses unsupported property %q", k)
		}
	}
	return s
}

func num(t *testing.T, s string) float32 {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		t.Fatalf("bad number %q", s)
	}
	return float32(f)
}

type rect [4]float64 // x, y, w, h relative to the fixture root

func TestChromeConformance(t *testing.T) {
	fixtures := parseFixtures(t)
	if *chrome != "" {
		regenerate(t, fixtures)
	}
	b, err := os.ReadFile("testdata/chrome.json")
	if err != nil {
		t.Fatalf("%v (generate it with -chrome <browser>)", err)
	}
	var want map[string][]rect
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}

	pass := 0
	for _, f := range fixtures {
		root, nodes := toLayout(t, f)
		root.Calculate(float32(math.NaN()), float32(math.NaN()))
		got := absolute(root, nodes)
		exp, ok := want[f.name]
		if !ok {
			t.Errorf("%s: missing from chrome.json; regenerate with -chrome", f.name)
			continue
		}
		var bad []string
		for i := range exp {
			if nodes[i].Style.Hidden {
				continue // a hidden box has no meaningful position
			}
			for j := 0; j < 4; j++ {
				if math.Abs(got[i][j]-exp[i][j]) > tolerance {
					bad = append(bad, fmt.Sprintf("  node %d: got %v, chrome %v", i, fmt1(got[i]), fmt1(exp[i])))
					break
				}
			}
		}
		if len(bad) > 0 {
			t.Errorf("%s:\n%s", f.name, strings.Join(bad, "\n"))
		} else {
			pass++
		}
	}
	t.Logf("%d/%d fixtures match Chromium", pass, len(fixtures))
}

func absolute(root *Node, order []*Node) []rect {
	pos := map[*Node][2]float64{root: {0, 0}}
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, c := range n.Children {
			p := pos[n]
			pos[c] = [2]float64{p[0] + float64(c.X), p[1] + float64(c.Y)}
			walk(c)
		}
	}
	walk(root)
	out := make([]rect, len(order))
	for i, n := range order {
		p := pos[n]
		out[i] = rect{p[0], p[1], float64(n.W), float64(n.H)}
	}
	return out
}

func fmt1(r rect) string {
	return fmt.Sprintf("[%g %g %g %g]", round(r[0]), round(r[1]), round(r[2]), round(r[3]))
}

func round(v float64) float64 { return math.Round(v*100) / 100 }

func regenerate(t *testing.T, fixtures []*htmlNode) {
	var page strings.Builder
	page.WriteString("<!doctype html><html><head><style>" + harnessCSS + "</style></head><body>\n")
	var write func(h *htmlNode)
	write = func(h *htmlNode) {
		fmt.Fprintf(&page, `<div style="%s">`, h.style)
		for _, c := range h.children {
			write(c)
		}
		page.WriteString("</div>")
	}
	for _, f := range fixtures {
		fmt.Fprintf(&page, `<section data-name="%s">`, f.name)
		write(f)
		page.WriteString("</section>\n")
	}
	page.WriteString(`<pre id="out"></pre><script>
const out = {};
for (const s of document.querySelectorAll("section")) {
  const root = s.firstElementChild, r0 = root.getBoundingClientRect();
  out[s.dataset.name] = [root, ...root.querySelectorAll("div")].map(d => {
    const r = d.getBoundingClientRect();
    return [r.left - r0.left, r.top - r0.top, r.width, r.height];
  });
}
document.getElementById("out").textContent = "@@" + JSON.stringify(out) + "@@";
</script></body></html>`)

	dir := t.TempDir()
	html := filepath.Join(dir, "fixtures.html")
	if err := os.WriteFile(html, []byte(page.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	// A brand-new profile makes the browser's first headless run hang on
	// first-run setup, so warm the throwaway profile up on a blank page
	// first. Both runs have hard timeouts.
	browse := func(url string, limit time.Duration) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), limit)
		defer cancel()
		return exec.CommandContext(ctx, *chrome, "--headless", "--disable-gpu", "--no-first-run",
			"--no-default-browser-check", "--disable-extensions", "--disable-component-update",
			"--user-data-dir="+filepath.Join(dir, "profile"), "--dump-dom", url).Output()
	}
	browse("about:blank", 20*time.Second) // expected to time out on a fresh profile
	dom, err := browse("file://"+html, 60*time.Second)
	if err != nil {
		t.Fatalf("browser: %v", err)
	}
	m := regexp.MustCompile(`@@(.*?)@@`).FindSubmatch(dom) // the first match is the <pre>; the script's own text comes later
	if m == nil {
		t.Fatalf("no results in the browser's DOM:\n%.500s", dom)
	}
	var res map[string][]rect
	if err := json.Unmarshal(m[1], &res); err != nil {
		t.Fatal(err)
	}
	// Stable, readable golden file: one fixture per line.
	names := make([]string, 0, len(res))
	for n := range res {
		names = append(names, n)
	}
	sort.Strings(names)
	var out strings.Builder
	out.WriteString("{\n")
	for i, n := range names {
		rs, _ := json.Marshal(res[n])
		fmt.Fprintf(&out, "%q: %s", n, rs)
		if i < len(names)-1 {
			out.WriteString(",")
		}
		out.WriteString("\n")
	}
	out.WriteString("}\n")
	if err := os.WriteFile("testdata/chrome.json", []byte(out.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("regenerated testdata/chrome.json from %d fixtures", len(names))
}
