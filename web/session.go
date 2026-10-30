package web

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/CodeSyncr/nimbus-native/layout"
	"github.com/CodeSyncr/nimbus-native/router"
	"github.com/CodeSyncr/nimbus-native/ui"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// session is one page view: the app's UI running on the server for one
// browser tab.
type session struct {
	id   string
	site *site
	ui   *ui.Session

	mu       sync.Mutex
	tr       *translator
	dom      *dom                    // the page until it is served
	styles   map[uint32]layout.Style // layout styles for the commit being built
	served   bool
	queue    [][]byte    // updates waiting for the page to connect
	ch       chan []byte // the connected page's event stream
	lastSeen time.Time

	path    string
	setPath func(string)
	nav     *navUpdate // a navigation to tell the page about with the next commit
}

type navUpdate struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	Push  bool   `json:"push"`
}

func newSession(id, path string, s *site) *session {
	sess := &session{id: id, site: s, tr: newTranslator(), dom: newDOM(), styles: map[uint32]layout.Style{},
		lastSeen: time.Now(), path: path}
	sess.ui = ui.NewSession(webRoot{sess}, ui.SessionOptions{
		Send:   sess.commit,
		Style:  func(tag uint32, st layout.Style) { sess.styles[tag] = st }, // on the UI loop, like commit
		Values: map[any]any{router.NavigatorKey: sess},
	})
	return sess
}

// webRoot shows the page for the session's path inside the site's layout.
type webRoot struct{ s *session }

func (r webRoot) Render(c *ui.Ctx) *ui.Element {
	path, setPath := ui.State(c, r.s.path)
	r.s.mu.Lock()
	r.s.setPath = setPath
	r.s.mu.Unlock()
	var page ui.Component = notFound{}
	if comp, _, ok := router.Match(path); ok {
		page = comp
	}
	var shown ui.Component = pageFrame{page: page, path: path}
	if r.s.site.o.Layout != nil {
		shown = r.s.site.o.Layout(path, shown)
	}
	return ui.C(shown)
}

// pageFrame holds the page, keyed by its path so each URL starts fresh.
type pageFrame struct {
	page ui.Component
	path string
}

func (p pageFrame) Render(c *ui.Ctx) *ui.Element {
	return ui.View(ui.Style{Grow: 1, Shrink: 1}, ui.C(p.page).Key(p.path))
}

type notFound struct{}

func (notFound) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	return ui.View(ui.Style{Grow: 1, Justify: ui.JustifyCenter, AlignItems: ui.AlignCenter, Padding: ui.All(th.SpaceL), Gap: th.SpaceM},
		ui.Text("Page not found", ui.Style{FontSize: th.Title, FontWeight: 700}),
		ui.Text("There is nothing at this address.", ui.Style{Color: th.TextMuted}),
		router.Link("/", ui.Style{Padding: ui.XY(16, 10)}, ui.Text("Go home", ui.Style{Color: th.Primary, FontWeight: 600})),
	)
}

// Navigate is router.Push on the web: show path's page and put it in the
// browser's history. It runs on the UI loop.
func (s *session) Navigate(path string) bool {
	return s.goTo(path, true)
}

func (s *session) goTo(path string, push bool) bool {
	comp, _, ok := router.Match(path)
	if !ok {
		return false
	}
	s.mu.Lock()
	set := s.setPath
	s.path = path
	s.nav = &navUpdate{Path: path, Title: s.site.title(router.HeadOf(comp, path)), Push: push}
	s.mu.Unlock()
	if set != nil {
		set(path)
	}
	return true
}

func (s *site) title(h router.Head) string {
	if s.o.SiteName == "" {
		return h.Title
	}
	if h.Title == "" || h.Title == s.o.SiteName {
		return s.o.SiteName
	}
	return h.Title + " · " + s.o.SiteName
}

// commit takes a render from the UI loop: into the page's HTML until it is
// served, then onto its event stream.
func (s *session) commit(b []byte) {
	m, err := render.Decode(b)
	if err != nil {
		return
	}
	c, ok := m.(*render.Commit)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	in := s.tr.commit(c, s.styles)
	s.styles = map[uint32]layout.Style{}
	if !s.served {
		s.dom.apply(in)
		s.nav = nil // the HTML is already at its address
		return
	}
	msg, err := json.Marshal(struct {
		I   []instr    `json:"i"`
		Nav *navUpdate `json:"nav,omitempty"`
	}{in, s.nav})
	s.nav = nil
	if err != nil {
		return
	}
	s.deliver(msg)
}

// deliver sends to the connected page, or queues until it connects.
// Caller holds mu.
func (s *session) deliver(msg []byte) {
	if s.ch == nil {
		s.queue = append(s.queue, msg)
		return
	}
	select {
	case s.ch <- msg:
	default:
		// The page fell too far behind: end its stream; it reloads.
		close(s.ch)
		s.ch = nil
	}
}

func (s *session) attach() chan []byte {
	ch := make(chan []byte, 1024)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch != nil {
		close(s.ch) // a newer connection for the same page wins
	}
	s.ch = ch
	if s.dom != nil {
		// The page's first connection: give it each element's style parts,
		// which its HTML carries only combined, before any later updates.
		var in []instr
		for tag, n := range s.dom.nodes {
			for _, k := range []string{"l", "b", "x"} {
				if css := n.style[k]; css != "" {
					in = append(in, instr{"s": []any{tag, k, css}})
				}
			}
		}
		if msg, err := json.Marshal(struct {
			I []instr `json:"i"`
		}{in}); err == nil {
			ch <- msg
		}
		s.dom = nil
	}
	for _, m := range s.queue {
		select {
		case ch <- m:
		default:
		}
	}
	s.queue = nil
	return ch
}

func (s *session) detach(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch == ch {
		s.ch = nil
		s.lastSeen = time.Now()
	}
}

func (s *session) idleFor() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch != nil {
		return 0
	}
	return time.Since(s.lastSeen)
}

func (s *session) close() { s.ui.Close() }

// clientMessage is an event from host.js.
type clientMessage struct {
	Phase uint8   `json:"phase"`
	PX    float32 `json:"x"`
	PY    float32 `json:"y"`
	PVX   float32 `json:"vx"`
	PVY   float32 `json:"vy"`
	T     string  `json:"t"`
	Tag   uint32  `json:"tag"`
	Text  string  `json:"text"`
	On    bool    `json:"on"`
	Index uint32  `json:"index"`
	Depth uint32  `json:"depth"`
	W     float32 `json:"w"`
	H     float32 `json:"h"`
	Scale float32 `json:"scale"`
	Dark  bool    `json:"dark"`
	Path  string  `json:"path"`
	Value float32 `json:"value"`
}

func (s *session) receive(m clientMessage) {
	var msg render.Message
	switch m.T {
	case "viewport":
		msg = &render.Viewport{W: m.W, H: m.H, Scale: m.Scale, Platform: "web", Dark: m.Dark}
	case "press":
		msg = &render.Press{Tag: m.Tag}
	case "changed":
		msg = &render.Changed{Tag: m.Tag, Text: m.Text}
	case "submitted":
		msg = &render.Submitted{Tag: m.Tag}
	case "toggled":
		msg = &render.Toggled{Tag: m.Tag, On: m.On}
	case "slid":
		msg = &render.Slid{Tag: m.Tag, Value: m.Value}
	case "longpress":
		msg = &render.LongPressed{Tag: m.Tag}
	case "panned":
		msg = &render.Panned{Tag: m.Tag, Phase: m.Phase, X: m.PX, Y: m.PY, VX: m.PVX, VY: m.PVY}
	case "pinched":
		msg = &render.Pinched{Tag: m.Tag, Phase: m.Phase, Scale: m.Scale}
	case "popped":
		msg = &render.Popped{Tag: m.Tag, Depth: m.Depth}
	case "selected":
		msg = &render.Selected{Tag: m.Tag, Index: m.Index}
	case "dismissed":
		msg = &render.Dismissed{Tag: m.Tag}
	case "navigate": // the browser's back and forward buttons
		s.ui.Do(func() { s.goTo(m.Path, false) })
		return
	default:
		return
	}
	s.ui.Receive(render.Encode(msg))
}
