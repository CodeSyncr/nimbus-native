// Package webview embeds a web page in your screen. For a page people visit
// and close, browser is simpler and safer.
//
//	nimbus native plugin add webview
//
//	webview.View{URL: "https://example.com/menu", Style: ui.Style{Grow: 1}}
package webview

import (
	"encoding/json"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// View is a web page.
type View struct {
	URL    string           // changing it loads the new page
	HTML   string           // or a page of your own, when URL is empty
	OnLoad func(url string) // a page finished loading
	Style  ui.Style
}

// Render shows the page.
func (v View) Render(c *ui.Ctx) ui.Component {
	return ui.Native("webview", map[string]string{"url": v.URL, "html": v.HTML}, v.Style).OnEvent(func(name, data string) {
		var e struct {
			URL string `json:"url"`
		}
		if name == "load" && v.OnLoad != nil && json.Unmarshal([]byte(data), &e) == nil {
			v.OnLoad(e.URL)
		}
	})
}
