// Package web serves a Nimbus Native app's pages to browsers: the same
// components the phone apps run, rendered on the server.
//
// A request for a page runs the app's UI on the server until its data has
// loaded and answers with plain HTML, with the page's title and meta tags,
// so search engines and link previews see real content. A small script
// (host.js, 8 KB, 3 KB gzipped) then takes over that HTML: taps and typing go to the
// page's UI session on the server, and the changes it renders come back as
// DOM updates. Layout is the browser's own CSS flexbox.
//
// Mount it on a Nimbus app as the fallback for everything the API doesn't
// serve:
//
//	site := web.Handler(web.Options{SiteName: "Shop", Layout: app.WebLayout, Home: "/orders"})
//	app.Router.Fallback(func(c *nhttp.Context) error { site.ServeHTTP(c.Response, c.Request); return nil })
//
// Pages come from the app's router table (router.Register); a page's
// Head() method gives its title, description and preview image.
package web

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/CodeSyncr/nimbus-native/router"
	"github.com/CodeSyncr/nimbus-native/ui"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Options configure the site.
type Options struct {
	// SiteName follows page titles ("Order #42 · Shop").
	SiteName string
	// Layout wraps every page: the web's shell (a header, navigation),
	// where the phones have tabs and stacks. Nil shows pages alone.
	Layout func(path string, page ui.Component) ui.Component
	// Home is where "/" redirects when no page has that path.
	Home string
	// BaseURL is the site's public address ("https://shop.example"), for
	// canonical links and the sitemap; by default the request's host.
	BaseURL string
	// Lang is the page language (default "en").
	Lang string
	// SettleTimeout bounds how long a page waits for its data before the
	// HTML goes out anyway (default 3s).
	SettleTimeout time.Duration
	// FontDir holds the app's bundled fonts (<family>.ttf or .otf), served
	// to browsers for Style.FontFamily. New projects use the Android app's
	// font folder.
	FontDir string
}

// Handler serves pages, the host script and the session endpoints.
func Handler(o Options) http.Handler {
	if o.Lang == "" {
		o.Lang = "en"
	}
	if o.SettleTimeout == 0 {
		o.SettleTimeout = 3 * time.Second
	}
	s := &site{o: o, sessions: map[string]*session{}}
	go s.reap()
	return s
}

type site struct {
	o        Options
	mu       sync.Mutex
	sessions map[string]*session
}

//go:embed host.js
var hostJS []byte

// hostVersion changes whenever host.js does, so browsers can cache it for
// good and still pick up a new one.
var hostVersion = func() string { h := sha256.Sum256(hostJS); return hex.EncodeToString(h[:4]) }()

const prefix = "/_nn/"

func (s *site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == prefix+"host.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // the URL carries its version
		w.Write(hostJS)
	case strings.HasPrefix(r.URL.Path, prefix+"fonts/") && s.o.FontDir != "":
		name := path.Base(r.URL.Path)
		if ext := path.Ext(name); ext != ".ttf" && ext != ".otf" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, filepath.Join(s.o.FontDir, name))
	case r.URL.Path == prefix+"events":
		s.events(w, r)
	case r.URL.Path == prefix+"send" && r.Method == http.MethodPost:
		s.send(w, r)
	case r.URL.Path == "/sitemap.xml":
		s.sitemap(w, r)
	case r.URL.Path == "/robots.txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", s.base(r))
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		s.page(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *site) base(r *http.Request) string {
	if s.o.BaseURL != "" {
		return strings.TrimRight(s.o.BaseURL, "/")
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// page renders a page on the server and answers with its HTML.
func (s *site) page(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	comp, _, found := router.Match(path)
	if !found && path == "/" && s.o.Home != "" {
		http.Redirect(w, r, s.o.Home, http.StatusFound)
		return
	}
	sess := s.newSession(path)
	sess.ui.Receive(render.Encode(&render.Viewport{W: 1280, H: 800, Scale: 1, Platform: "web"}))
	sess.ui.Settle(s.o.SettleTimeout)

	sess.mu.Lock()
	var body strings.Builder
	sess.dom.html(&body)
	theme := sess.dom.themeCSS()
	sess.served = true // from now on, commits queue for the browser
	sess.mu.Unlock()

	head := router.Head{Title: "Not found", NoIndex: true}
	if found {
		head = router.HeadOf(comp, path)
	}
	status := http.StatusOK
	if !found {
		status = http.StatusNotFound
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store") // each page view has its own session
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	fmt.Fprintf(w, `<!doctype html>
<html lang="%s">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
%s
<style>%s%s%s</style>
</head>
<body>
<div id="nn">%s</div>
<script src="%shost.js?v=%s" data-session="%s" defer></script>
</body>
</html>
`, html.EscapeString(s.o.Lang), s.headTags(head, s.base(r)+path), s.fontFaces(), baseCSS, theme, body.String(), prefix, hostVersion, sess.id)
}

// fontFaces declares the app's bundled fonts.
func (s *site) fontFaces() string {
	if s.o.FontDir == "" {
		return ""
	}
	files, _ := filepath.Glob(filepath.Join(s.o.FontDir, "*"))
	var b strings.Builder
	for _, f := range files {
		name := filepath.Base(f)
		ext := filepath.Ext(name)
		if ext != ".ttf" && ext != ".otf" {
			continue
		}
		format := map[string]string{".ttf": "truetype", ".otf": "opentype"}[ext]
		fmt.Fprintf(&b, "@font-face{font-family:\"%s\";src:url(%sfonts/%s) format(\"%s\");font-weight:100 900;font-display:swap}\n",
			strings.TrimSuffix(name, ext), prefix, name, format)
	}
	return b.String()
}

// headTags is the page's title and meta tags.
func (s *site) headTags(h router.Head, url string) string {
	title := s.title(h)
	var b strings.Builder
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	meta := func(attr, name, content string) {
		if content != "" {
			fmt.Fprintf(&b, "<meta %s=\"%s\" content=\"%s\">\n", attr, name, html.EscapeString(content))
		}
	}
	meta("name", "description", h.Description)
	meta("property", "og:title", title)
	meta("property", "og:description", h.Description)
	meta("property", "og:image", h.Image)
	meta("property", "og:url", url)
	meta("property", "og:site_name", s.o.SiteName)
	if h.NoIndex {
		meta("name", "robots", "noindex")
	}
	fmt.Fprintf(&b, "<link rel=\"canonical\" href=\"%s\">", html.EscapeString(url))
	return b.String()
}

// sitemap lists every page whose path has no parameters.
func (s *site) sitemap(w http.ResponseWriter, r *http.Request) {
	var paths []string
	for _, p := range router.Patterns() {
		if !strings.Contains(p, ":") {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+"\n")
	for _, p := range paths {
		fmt.Fprintf(w, "<url><loc>%s</loc></url>\n", html.EscapeString(s.base(r)+p))
	}
	fmt.Fprint(w, "</urlset>\n")
}

// events streams a session's DOM updates to its page (Server-Sent Events).
func (s *site) events(w http.ResponseWriter, r *http.Request) {
	sess := s.lookup(r.URL.Query().Get("s"))
	flusher, ok := w.(http.Flusher)
	if sess == nil || !ok {
		http.Error(w, "session expired", http.StatusGone) // the script reloads the page
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	ch := sess.attach()
	defer sess.detach(ch)
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case msg, open := <-ch:
			if !open {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// send takes one event from the page (a tap, typing, the viewport).
func (s *site) send(w http.ResponseWriter, r *http.Request) {
	sess := s.lookup(r.URL.Query().Get("s"))
	if sess == nil {
		http.Error(w, "session expired", http.StatusGone)
		return
	}
	var m clientMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&m); err != nil {
		http.Error(w, "bad message", http.StatusBadRequest)
		return
	}
	sess.receive(m)
	w.WriteHeader(http.StatusNoContent)
}

func (s *site) newSession(path string) *session {
	b := make([]byte, 16)
	rand.Read(b)
	sess := newSession(hex.EncodeToString(b), path, s)
	s.mu.Lock()
	s.sessions[sess.id] = sess
	s.mu.Unlock()
	return sess
}

func (s *site) lookup(id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

// reap closes sessions whose page went away: never connected within a
// minute of rendering, or disconnected for a minute.
func (s *site) reap() {
	for range time.Tick(15 * time.Second) {
		s.mu.Lock()
		for id, sess := range s.sessions {
			if sess.idleFor() > time.Minute {
				delete(s.sessions, id)
				go sess.close()
			}
		}
		s.mu.Unlock()
	}
}

// baseCSS makes the browser lay views out the way the Go engine does.
const baseCSS = `*,*::before,*::after{box-sizing:border-box}
html,body{margin:0;height:100%;background:var(--bg,#fff);color:var(--fg,#111);font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;-webkit-font-smoothing:antialiased;font-size:15px}
#nn{position:fixed;inset:0;display:flex;flex-direction:column}
#nn>[data-t]{position:absolute!important;inset:0;width:auto!important;height:auto!important}
.v,.p,.sc,.st,.tb,.md{display:flex;flex-direction:column;position:relative;min-width:0;min-height:0;flex-shrink:0}
.t{white-space:pre-wrap;overflow-wrap:anywhere;min-width:0}
a.p{color:inherit;text-decoration:none;cursor:pointer;-webkit-tap-highlight-color:transparent;user-select:none}
a.p:active{opacity:.6}
a.p:focus-visible,.in:focus-visible{outline:2px solid var(--ac,#2563eb);outline-offset:2px}
.in{border:0;outline:none;background:transparent;font:inherit;color:inherit;padding:10px;resize:none;min-width:0}
textarea.in{line-height:1.3}
.i{display:block;min-width:0}
.sp{width:20px;height:20px;border:2px solid rgba(127,127,127,.25);border-top-color:var(--ac,#2563eb);border-radius:50%;animation:nnspin .8s linear infinite}
@keyframes nnspin{to{transform:rotate(360deg)}}
.sw{width:40px;height:22px;margin:0;cursor:pointer}
.sl{margin:0;min-width:0;cursor:pointer}
.st>[data-t]:not(:last-of-type),.tb>[data-t]:not(.sel){display:none!important}
.bar{display:flex;align-items:center;justify-content:space-between;gap:8px;height:52px;padding:0 12px;background:var(--bar,#fff);color:var(--barfg,#111);border-bottom:1px solid var(--border,#e5e7eb);font-weight:600;flex-shrink:0}
.bar button,.tabbar button{background:none;border:0;font:inherit;color:var(--ac,#2563eb);cursor:pointer;padding:8px}
.tabbar{display:flex;flex-shrink:0;background:var(--bar,#fff);border-top:1px solid var(--border,#e5e7eb)}
.tabbar button{flex:1;color:var(--muted,#6b7280);padding:14px 8px}
.tabbar button.on{color:var(--ac,#2563eb);font-weight:600}
.ov{position:fixed;inset:0;background:rgba(0,0,0,.4);display:flex;align-items:flex-end;justify-content:center;z-index:10}
.ov>.md{position:relative!important;left:auto!important;top:auto!important;width:100%!important;max-width:560px;height:85%!important;background:var(--bg,#fff);border-radius:14px 14px 0 0;overflow:hidden}
@media(min-width:700px){.ov{align-items:center}.ov>.md{border-radius:14px;height:min(85%,720px)!important}}
`
