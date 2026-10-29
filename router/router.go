// Package router maps URL paths to screens. With folder routing, a project's
// pages/ directory is the map: each folder holding a page.go is a route, and
// a folder named _name is a parameter.
//
//	pages/page.go               /
//	pages/orders/page.go        /orders
//	pages/orders/_id/page.go    /orders/:id
//
// `nimbus native gen` (run by every build) turns the folders into a static
// table, pages/routes_gen.go, so matching costs a few string compares and no
// reflection. Each page.go declares a Page component; parameters land in
// fields tagged param:
//
//	package order // pages/orders/_id
//
//	type Page struct {
//		ID int `param:"id"`
//	}
//
//	func (p Page) Title() string { return fmt.Sprintf("Order #%d", p.ID) } // optional
//	func (p Page) Render(c *ui.Ctx) *ui.Element { ... }
//
// Register the table once, then navigate by path from any screen:
//
//	router.Register(pages.Routes)
//	router.Push(c, "/orders/42")
package router

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Route is one page: its pattern ("/orders/:id") and how to make its
// component from the parameters, false when they don't parse.
type Route struct {
	Pattern string
	New     func(Params) (ui.Component, bool)
}

// Table is an app's routes.
type Table []Route

// Params are a matched path's parameters by name.
type Params map[string]string

// Get is a parameter's value ("" if absent).
func (p Params) Get(name string) string { return p[name] }

// Int is a parameter as an int; false when it isn't one.
func (p Params) Int(name string) (int, bool) {
	n, err := strconv.Atoi(p[name])
	return n, err == nil
}

// Int64 is a parameter as an int64; false when it isn't one.
func (p Params) Int64(name string) (int64, bool) {
	n, err := strconv.ParseInt(p[name], 10, 64)
	return n, err == nil
}

// Titled is a page that names its screen; Push uses it for the bar title.
type Titled interface{ Title() string }

type compiled struct {
	Route
	segs []string // ":id" for a parameter
}

var (
	mu     sync.RWMutex
	routes []compiled
)

// Register installs the app's routes, replacing any before. Call it from
// init, next to ui.Mount.
func Register(t Table) {
	cs := make([]compiled, len(t))
	for i, r := range t {
		cs[i] = compiled{Route: r, segs: split(r.Pattern)}
	}
	// Most specific first: at the first segment where two routes differ in
	// kind, the static one wins ("/orders/new" before "/orders/:id").
	sort.SliceStable(cs, func(i, j int) bool { return moreSpecific(cs[i].segs, cs[j].segs) })
	mu.Lock()
	routes = cs
	mu.Unlock()
	ui.RestoreRoute = func(path string) (string, ui.Component, bool) {
		comp, _, ok := Match(path)
		if !ok {
			return "", nil, false
		}
		return TitleOf(comp, path), comp, true
	}
}

// Match finds the page for a path ("/orders/42?tab=items"): its component
// and parameters, with query values added. false when no route matches.
func Match(path string) (ui.Component, Params, bool) {
	path, rawQuery, _ := strings.Cut(path, "?")
	segs := split(path)
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range routes {
		if len(r.segs) != len(segs) {
			continue
		}
		var params Params
		ok := true
		for i, s := range r.segs {
			if s[0] == ':' {
				v, err := url.PathUnescape(segs[i])
				if err != nil {
					ok = false
					break
				}
				if params == nil {
					params = Params{}
				}
				params[s[1:]] = v
			} else if s != segs[i] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if rawQuery != "" {
			if q, err := url.ParseQuery(rawQuery); err == nil {
				for k, v := range q {
					if params == nil {
						params = Params{}
					}
					if _, taken := params[k]; !taken && len(v) > 0 {
						params[k] = v[0]
					}
				}
			}
		}
		if c, ok := r.New(params); ok {
			return c, params, true
		}
	}
	return nil, nil, false
}

// Push shows the page at path on top of the Stack the component is in,
// titled by its Title method or else its last static path segment. Outside
// a Stack, on the web, it goes to that page and its URL. It reports false,
// and changes nothing, when no route matches.
func Push(c *ui.Ctx, path string) bool {
	comp, _, ok := Match(path)
	if !ok {
		return false
	}
	if nav := ui.Navigation(c); nav != nil {
		nav.PushPath(TitleOf(comp, path), path, comp)
		return true
	}
	if n, ok := ui.SessionValue(c, NavigatorKey).(Navigator); ok {
		return n.Navigate(path)
	}
	ui.Navigation(c).PushPath(TitleOf(comp, path), path, comp) // logs: no Stack here
	return false
}

// Navigator moves between pages where there is no Stack: the web host
// implements it to change the page and the browser's URL. A session puts it
// in its values under NavigatorKey.
type Navigator interface{ Navigate(path string) bool }

type navigatorKey struct{}

// NavigatorKey is the ui.SessionValue key of a session's Navigator.
var NavigatorKey any = navigatorKey{}

// Head is what a page tells search engines and link previews: the web
// host puts it in the page's <head>. Pages provide it with a method:
//
//	func (p Page) Head() router.Head {
//		return router.Head{Title: "Order #42", Description: "…"}
//	}
type Head struct {
	Title       string
	Description string
	Image       string // absolute URL of a preview image (og:image)
	NoIndex     bool   // keep the page out of search results
}

// HeadOf is a page's Head: its Head method, else its title.
func HeadOf(comp ui.Component, path string) Head {
	if h, ok := comp.(interface{ Head() Head }); ok {
		return h.Head()
	}
	return Head{Title: TitleOf(comp, path)}
}

// Link is a pressable that goes to path: router.Push when tapped, and on
// the web a real link that crawlers and new tabs follow.
//
//	router.Link("/orders/42", ui.Style{}, ui.Text("Order #42", ui.Style{}))
func Link(path string, s ui.Style, children ...*ui.Element) *ui.Element {
	return ui.C(link{path: path, style: s, children: children})
}

type link struct {
	path     string
	style    ui.Style
	children []*ui.Element
}

func (l link) Render(c *ui.Ctx) *ui.Element {
	return ui.Pressable(func() { Push(c, l.path) }, l.style, l.children...).Href(l.path)
}

// Patterns lists the routes' patterns, most specific first: the web host
// builds its sitemap from the ones without parameters.
func Patterns() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, len(routes))
	for i, r := range routes {
		out[i] = r.Pattern
	}
	return out
}

// TitleOf is a page's title: its Title method, else its path's last
// static segment, capitalized ("/orders/42" → "Orders").
func TitleOf(comp ui.Component, path string) string {
	if t, ok := comp.(Titled); ok {
		return t.Title()
	}
	path, _, _ = strings.Cut(path, "?")
	segs := split(path)
	for i := len(segs) - 1; i >= 0; i-- {
		if s := segs[i]; s != "" && !isDigits(s) {
			return strings.ToUpper(s[:1]) + strings.ReplaceAll(s[1:], "-", " ")
		}
	}
	return ""
}

// split is a path's segments, ignoring empty ones ("/a//b/" → a, b).
func split(path string) []string {
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

func moreSpecific(a, b []string) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if pa, pb := a[i][0] == ':', b[i][0] == ':'; pa != pb {
			return pb
		}
	}
	return false
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
