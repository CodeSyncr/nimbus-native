package ui

import (
	"strconv"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// ModalNav presents and dismisses modals: screens shown over everything —
// a sheet on iOS (swipe down to close), a full-screen layer on Android
// (back to close). Each modal is its own navigation stack with a title bar,
// so Navigation works inside it. Safe from any goroutine.
type ModalNav struct {
	rt *runtime
	in *instance // the modal the component is in, if any
}

// Modals presents and dismisses modals from any component.
func Modals(c *Ctx) *ModalNav {
	m := &ModalNav{rt: c.rt}
	for p := c.inst.parent; p != nil; p = p.parent {
		if p.tag != 0 && p.el.kind == render.KindModal {
			m.in = p
			break
		}
	}
	return m
}

// Present shows screen in a new modal titled title, over everything else.
func (m *ModalNav) Present(title string, screen Component) {
	rt := m.rt
	rt.do(func() {
		rt.modalKey++
		rt.modals = append(rt.modals, &modal{key: "m" + strconv.Itoa(rt.modalKey), title: title, comp: screen})
		rt.dirty = true
	})
}

// Dismiss closes the modal the component is in, or else the topmost one.
func (m *ModalNav) Dismiss() {
	rt, in := m.rt, m.in
	rt.do(func() {
		for i := len(rt.modals) - 1; i >= 0; i-- {
			if in == nil || rt.modals[i].inst == in {
				rt.closeModal(i)
				return
			}
		}
	})
}

// Count is how many modals are open.
func (m *ModalNav) Count() int { return len(m.rt.modals) }

type modal struct {
	key       string
	title     string
	comp      Component
	inst      *instance
	w, h      float32 // the size the host gives the modal
	presented bool
}

func (rt *runtime) closeModal(i int) {
	m := rt.modals[i]
	rt.modals = append(rt.modals[:i], rt.modals[i+1:]...)
	if m.inst != nil {
		rt.unmount(m.inst) // its Destroy dismisses it on the host
	}
	rt.dirty = true
}

// reconcileModals renders every open modal as its own tree and presents the
// new ones.
func (rt *runtime) reconcileModals() {
	for _, m := range rt.modals {
		w, h := m.w, m.h
		if w == 0 || h == 0 {
			// Until the host reports it: an iOS sheet is the screen less the
			// status bar and a sliver; Android's layer is the whole screen.
			w, h = rt.viewport.W, rt.viewport.H
			if rt.viewport.Platform != "android" {
				h -= rt.viewport.SafeTop + 10
			}
		}
		el := &Element{kind: render.KindModal, key: m.key, style: Style{Width: Pt(w), Height: Pt(h)},
			children: []*Element{Stack(m.title, m.comp, Flex(1))}}
		m.inst = rt.reconcile(m.inst, el, nil)
		if !m.presented {
			m.presented = true
			rt.op(&render.Present{Tag: m.inst.tag})
		}
	}
}

// layoutModals lays out each modal at its size, after the main tree.
func (rt *runtime) layoutModals() {
	for _, m := range rt.modals {
		if m.inst == nil {
			continue
		}
		m.inst.node.Calculate(m.inst.node.Style.Width.V, m.inst.node.Style.Height.V)
		rt.emitFrames(m.inst)
	}
}

// modalResized gives a modal the size the host reports; false if in is not a modal.
func (rt *runtime) modalResized(in *instance, w, h float32) bool {
	for _, m := range rt.modals {
		if m.inst == in {
			if m.w != w || m.h != h {
				m.w, m.h = w, h
				rt.dirty = true
			}
			return true
		}
	}
	return false
}

func (rt *runtime) dismissed(tag uint32) {
	for i, m := range rt.modals {
		if m.inst != nil && m.inst.tag == tag {
			rt.closeModal(i)
			return
		}
	}
}
