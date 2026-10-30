package ui

import (
	"net/url"
	"strconv"
	"strings"
)

// Links that open the app (myapp://orders/42; web links the app handles)
// arrive from the host. Each becomes a path for the app's routes, and its
// page opens: pushed on the navigation stack on screen, or shown as a sheet
// when the screen has no stack.

// HandleURL, when set, sees every link first; returning true means it was
// handled and nothing else happens.
var HandleURL func(link string) bool

// LinkPath is the route path a link points to: a web link's path, or for
// an app's own scheme the host and path together (myapp://orders/42 is
// /orders/42).
func LinkPath(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	p := u.Path
	if u.Scheme != "http" && u.Scheme != "https" && u.Host != "" {
		p = "/" + u.Host + p
	}
	if p == "" {
		p = "/"
	}
	if u.RawQuery != "" {
		p += "?" + u.RawQuery
	}
	return p
}

// openLink opens a link's page; before the first render it waits for one.
func (rt *runtime) openLink(link string) {
	if HandleURL != nil && HandleURL(link) {
		return
	}
	if rt.rootInst == nil {
		rt.pendingLink = link
		return
	}
	path := LinkPath(link)
	if path == "" || strings.Split(path, "?")[0] == "/" || RestoreRoute == nil {
		return // the app's home: it is already open
	}
	title, comp, ok := RestoreRoute(path)
	if !ok {
		logf("ui: no page for link %s", link)
		return
	}
	if in := rt.visibleStack(rt.rootInst); in != nil && in.stack != nil {
		st := in.stack
		st.nextKey++
		st.screens = append(st.screens, screen{title: title, comp: comp, path: path, key: "s" + strconv.Itoa(st.nextKey)})
		rt.rechild[in] = true
	} else {
		rt.modalKey++
		rt.modals = append(rt.modals, &modal{key: "m" + strconv.Itoa(rt.modalKey), title: title, comp: comp})
	}
	rt.dirty = true
}

// visibleStack is the innermost Stack on screen: inside Tabs, the selected
// tab's.
func (rt *runtime) visibleStack(in *instance) *instance {
	if in == nil || in.unmounted {
		return nil
	}
	if in.tabs != nil {
		if i := in.tabs.selected; i < len(in.children) {
			return rt.visibleStack(in.children[i])
		}
		return nil
	}
	var found *instance
	if in.stack != nil {
		found = in
		if n := len(in.children); n > 0 {
			if deeper := rt.visibleStack(in.children[n-1]); deeper != nil {
				return deeper
			}
		}
		return found
	}
	if d := rt.visibleStack(in.child); d != nil {
		return d
	}
	for _, c := range in.children {
		if d := rt.visibleStack(c); d != nil {
			return d
		}
	}
	return nil
}
