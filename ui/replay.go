package ui

import "github.com/CodeSyncr/nimbus-native/ui/render"

// replay draws the whole UI again into a new native view hierarchy (an
// Android activity created again while the process lived on). Components
// keep their state; every view is created, styled, placed and titled anew.
func (rt *runtime) replay() {
	if rt.rootInst == nil {
		return
	}
	rt.sentChrome = render.Chrome{} // the new views have no theme yet
	rt.rootTag = 0                  // and no root
	rt.replayTree(rt.rootInst)
	for _, m := range rt.modals {
		if m.inst != nil {
			rt.replayTree(m.inst)
			rt.op(&render.Present{Tag: m.inst.tag})
		}
	}
	rt.forceAll, rt.dirty = true, true // frames and the rest follow in the next commit
}

func (rt *runtime) replayTree(in *instance) {
	if in == nil || in.unmounted {
		return
	}
	if in.tag != 0 {
		rt.op(&render.Create{Tag: in.tag, Kind: in.el.kind})
		in.sent = sentProps{} // send every property again
		rt.props(in, true)
	}
	rt.replayTree(in.child)
	for _, c := range in.children {
		rt.replayTree(c)
	}
	if in.tag == 0 {
		return
	}
	if len(in.childTags) > 0 {
		rt.op(&render.Children{Tag: in.tag, Children: in.childTags})
	}
	for _, c := range in.children {
		r := hostRoot(c)
		switch {
		case r == nil:
		case in.stack != nil && r.title != "":
			rt.op(&render.Screen{Tag: r.tag, Title: r.title})
		case in.tabs != nil:
			rt.op(&render.TabItem{Tag: r.tag, Title: r.title, Icon: r.icon})
		}
	}
	if in.tabs != nil {
		in.tabs.sent = in.tabs.selected
		rt.op(&render.TabSelect{Tag: in.tag, Index: uint32(in.tabs.selected)})
	}
}
