package ui

import (
	"encoding/json"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Native shows a view written in Swift and Kotlin, inside your screen: a
// map, a video, a view from another SDK. The host registers a maker for kind
// (NativeViews.register on both platforms), props reach it as JSON whenever
// they change, and Style sizes it like any view. Plugins wrap this in a
// component of their own; see plugins/googlemaps.
func Native(kind string, props any, s Style) *Element {
	b, err := json.Marshal(props)
	if err != nil {
		logf("ui.Native %s: %v", kind, err)
		b = []byte("null")
	}
	return &Element{kind: render.KindNative, style: s, native: &render.Native{Type: kind, Props: string(b)}}
}

// OnEvent gets what a Native view reports: a name, and JSON.
func (e *Element) OnEvent(f func(name, data string)) *Element { e.onEvent = f; return e }
