package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/CodeSyncr/nimbus-native/router"
	"github.com/CodeSyncr/nimbus-native/ui"
)

type ordersPage struct{}

func (ordersPage) Head() router.Head {
	return router.Head{Title: "Orders", Description: "Every order, newest first."}
}

func (ordersPage) Render(c *ui.Ctx) *ui.Element {
	q := ui.UseQuery(c, "orders", func(ctx context.Context) ([]string, error) {
		time.Sleep(30 * time.Millisecond) // data the HTML must wait for
		return []string{"Asha Rao", "Vikram Shah"}, nil
	})
	if !q.Loaded {
		return ui.Spinner(ui.Style{})
	}
	rows := []ui.Component{}
	for i, name := range q.Data {
		rows = append(rows, router.Link(fmt.Sprintf("/orders/%d", i+1), ui.Style{}, ui.Text(name, ui.Style{})).TestID(fmt.Sprint("row-", i+1)))
	}
	return ui.View(ui.Style{Gap: 8}, rows...)
}

type orderPage struct{ ID int }

func (p orderPage) Head() router.Head { return router.Head{Title: fmt.Sprintf("Order #%d", p.ID)} }

func (p orderPage) Render(c *ui.Ctx) *ui.Element {
	n, setN := ui.State(c, 0)
	return ui.View(ui.Style{},
		ui.Text(fmt.Sprintf("Order %d, tapped %d", p.ID, n), ui.Style{FontWeight: 700}).TestID("detail"),
		ui.Pressable(func() { setN(n + 1) }, ui.Style{Padding: ui.All(8)}, ui.Text("Tap", ui.Style{})).TestID("tap"),
	)
}

type shell struct {
	path string
	page ui.Component
}

func (s shell) Render(c *ui.Ctx) *ui.Element {
	return ui.View(ui.Style{Grow: 1}, ui.Text("Shop", ui.Style{FontSize: 20}).TestID("brand"), ui.C(s.page))
}

func testSite(t *testing.T) *httptest.Server {
	t.Helper()
	router.Register(router.Table{
		{Pattern: "/orders", New: func(router.Params) (ui.Component, bool) { return ordersPage{}, true }},
		{Pattern: "/orders/:id", New: func(p router.Params) (ui.Component, bool) {
			id, ok := p.Int("id")
			return orderPage{id}, ok
		}},
	})
	srv := httptest.NewServer(Handler(Options{SiteName: "Shop", Home: "/orders",
		Layout: func(path string, page ui.Component) ui.Component { return shell{path, page} }}))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r, string(b)
}

func TestServerRendering(t *testing.T) {
	srv := testSite(t)
	r, body := get(t, srv.URL+"/orders")
	if r.StatusCode != 200 {
		t.Fatalf("status %d", r.StatusCode)
	}
	for _, want := range []string{
		"<title>Orders · Shop</title>",
		`<meta name="description" content="Every order, newest first.">`,
		`<link rel="canonical" href="` + srv.URL + `/orders">`,
		">Shop</div>",                  // the layout
		">Asha Rao</div>",              // data the page waited for
		`href="/orders/2"`,             // links crawlers can follow
		`data-testid="row-1"`,          // attributes
		"display:flex;flex-direction:", // layout as CSS
		`<script src="/_nn/host.js?v=` + hostVersion + `" data-session="`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %s", want)
		}
	}
	if strings.Contains(body, `class="sp"`) {
		t.Error("the page went out with its spinner, before its data")
	}

	if r, _ := get(t, srv.URL+"/"); r.StatusCode != http.StatusFound || r.Header.Get("Location") != "/orders" {
		t.Errorf("home: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	if r, body := get(t, srv.URL+"/nope"); r.StatusCode != 404 || !strings.Contains(body, "Page not found") || !strings.Contains(body, "noindex") {
		t.Errorf("missing page: %d", r.StatusCode)
	}
	if _, body := get(t, srv.URL+"/sitemap.xml"); !strings.Contains(body, "<loc>"+srv.URL+"/orders</loc>") || strings.Contains(body, ":id") {
		t.Errorf("sitemap:\n%s", body)
	}
	if _, body := get(t, srv.URL+"/robots.txt"); !strings.Contains(body, "Sitemap: "+srv.URL+"/sitemap.xml") {
		t.Errorf("robots.txt:\n%s", body)
	}
	if r, body := get(t, srv.URL+"/_nn/host.js"); r.StatusCode != 200 || !strings.Contains(body, "EventSource") {
		t.Error("no host script")
	}
}

// A tap travels to the session and the change comes back on the stream; a
// link's tap navigates, with the browser told the new address.
func TestLiveSession(t *testing.T) {
	srv := testSite(t)
	_, body := get(t, srv.URL+"/orders/7")
	sid := regexp.MustCompile(`data-session="([0-9a-f]+)"`).FindStringSubmatch(body)[1]
	tagOf := func(html, id string) int {
		m := regexp.MustCompile(`data-t="(\d+)"[^>]*data-testid="` + id + `"`).FindStringSubmatch(html)
		if m == nil {
			t.Fatalf("no %s in %s", id, html)
		}
		var n int
		fmt.Sscan(m[1], &n)
		return n
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/_nn/events?s="+sid, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	events := make(chan string, 32)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				events <- data
			}
		}
	}()
	waitFor := func(substr string) string {
		t.Helper()
		deadline := time.After(3 * time.Second)
		for {
			select {
			case e := <-events:
				if strings.Contains(e, substr) {
					return e
				}
			case <-deadline:
				t.Fatalf("no event with %q", substr)
				return ""
			}
		}
	}
	post := func(m map[string]any) {
		b, _ := json.Marshal(m)
		r, err := http.Post(srv.URL+"/_nn/send?s="+sid, "application/json", strings.NewReader(string(b)))
		if err != nil || r.StatusCode != http.StatusNoContent {
			t.Fatalf("send: %v %v", err, r)
		}
	}

	waitFor(`"s":[`) // the style parts, first
	post(map[string]any{"t": "viewport", "w": 400, "h": 700, "scale": 2})
	post(map[string]any{"t": "press", "tag": tagOf(body, "tap")})
	waitFor("Order 7, tapped 1")

	// Back to the list through the browser's history, then a link.
	post(map[string]any{"t": "navigate", "path": "/orders"})
	list := waitFor("Asha Rao")
	m := regexp.MustCompile(`"a":\[(\d+),"data-testid","row-2"\]`).FindStringSubmatch(list)
	if m == nil {
		t.Fatalf("no row-2 in %s", list)
	}
	var row int
	fmt.Sscan(m[1], &row)
	post(map[string]any{"t": "press", "tag": row})
	nav := waitFor(`"nav"`)
	if !strings.Contains(nav, `"path":"/orders/2","title":"Order #2 · Shop","push":true`) || !strings.Contains(nav, "Order 2, tapped 0") {
		t.Fatalf("link tap: %s", nav)
	}
	if r, _ := http.Post(srv.URL+"/_nn/send?s=nope", "application/json", strings.NewReader(`{"t":"press"}`)); r.StatusCode != http.StatusGone {
		t.Errorf("unknown session: %d", r.StatusCode)
	}
}
