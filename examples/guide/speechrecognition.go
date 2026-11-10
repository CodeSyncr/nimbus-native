package guide

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/speechrecognition"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// VoiceSearch writes out what they say, for up to ten seconds.
type VoiceSearch struct{}

func (VoiceSearch) Render(c *ui.Ctx) ui.Component {
	heard, setHeard := ui.State(c, "")
	listen := func() {
		go func() {
			ctx, stop := context.WithTimeout(c.Context(), 10*time.Second)
			defer stop()
			if ok, err := speechrecognition.Request(ctx); err != nil || !ok {
				setHeard("The microphone is off")
				return
			}
			text, err := speechrecognition.Listen(ctx, speechrecognition.Options{Language: "en-IN", OnPartial: setHeard})
			if err != nil {
				setHeard(err.Error())
				return
			}
			setHeard(text)
		}()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Speak", OnPress: listen},
		ui.Text(heard, ui.Style{FontSize: 20}))
}
