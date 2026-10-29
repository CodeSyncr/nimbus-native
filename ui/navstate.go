package ui

import (
	"os"
	"strconv"
	"strings"
)

// Dev reloads restart the app process. Under `nimbus native dev`, which sets
// NIMBUS_DEV_STATE, the runtime saves which tab each Tabs shows and the
// paths of the screens pushed on each Stack, and the next process opens
// them again: an edit keeps you on the screen you were looking at. Screens
// pushed with a path (router.Push) come back; a stack stops at the first
// one without. Outside dev mode none of this runs.

// RestoreRoute turns a path back into a screen. router.Register sets it.
var RestoreRoute func(path string) (title string, screen Component, ok bool)

// navState is what a dev reload carries over, in the order the Tabs and
// Stacks were first mounted.
type navState struct {
	tabs   []int
	stacks [][]string
}

// loadNav reads the state a previous process saved, if any.
func (rt *runtime) loadNav(file string) {
	rt.navFile = file
	b, err := os.ReadFile(file)
	if err != nil {
		return
	}
	rt.navSaved = parseNav(string(b))
	rt.navLast = string(b)
}

func parseNav(s string) *navState {
	st := &navState{}
	for _, line := range strings.Split(s, "\n") {
		f := strings.Split(line, "\t")
		switch f[0] {
		case "tabs":
			for _, v := range f[1:] {
				n, _ := strconv.Atoi(v)
				st.tabs = append(st.tabs, n)
			}
		case "stack":
			st.stacks = append(st.stacks, f[1:])
		}
	}
	return st
}

// savedTab is the tab to select for the next Tabs mounted, or 0.
func (rt *runtime) savedTab() int {
	i := rt.navTabs
	rt.navTabs++
	if rt.navSaved == nil || i >= len(rt.navSaved.tabs) {
		return 0
	}
	return rt.navSaved.tabs[i]
}

// savedScreens rebuilds the pushed screens for the next Stack mounted.
func (rt *runtime) savedScreens(st *stackState) {
	i := rt.navStacks
	rt.navStacks++
	if rt.navSaved == nil || i >= len(rt.navSaved.stacks) || RestoreRoute == nil {
		return
	}
	for _, path := range rt.navSaved.stacks[i] {
		title, comp, ok := RestoreRoute(path)
		if !ok {
			return
		}
		st.nextKey++
		st.screens = append(st.screens, screen{title: title, comp: comp, path: path, key: "s" + strconv.Itoa(st.nextKey)})
	}
}

// saveNav writes the current tabs and stacks when they changed.
func (rt *runtime) saveNav() {
	if rt.navFile == "" || rt.rootInst == nil {
		return
	}
	var b strings.Builder
	var tabs []string
	var walk func(in *instance)
	walk = func(in *instance) {
		if in == nil || in.unmounted {
			return
		}
		switch {
		case in.tabs != nil:
			tabs = append(tabs, strconv.Itoa(in.tabs.selected))
		case in.stack != nil:
			b.WriteString("stack")
			for _, sc := range in.stack.screens[1:] {
				if sc.path == "" {
					break
				}
				b.WriteString("\t" + sc.path)
			}
			b.WriteString("\n")
		}
		walk(in.child)
		for _, c := range in.children {
			walk(c)
		}
	}
	walk(rt.rootInst)
	s := "tabs\t" + strings.Join(tabs, "\t") + "\n" + b.String()
	if s == rt.navLast {
		return
	}
	rt.navLast = s
	if err := os.WriteFile(rt.navFile, []byte(s), 0o644); err != nil {
		logf("dev: saving navigation: %v", err)
	}
}
