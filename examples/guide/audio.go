package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/audio"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// AskOnce listens for one spoken turn and plays it back. A real app sends
// the WAV to its server and plays the answer instead.
type AskOnce struct{}

func (AskOnce) Render(c *ui.Ctx) ui.Component {
	status, setStatus := ui.State(c, "Tap and speak")
	listen := func() {
		go func() {
			ctx := c.Context()
			if ok, err := audio.RequestMicrophone(ctx); err != nil || !ok {
				setStatus("The microphone is off")
				return
			}
			defer audio.Release()
			setStatus("Listening…")
			l := &audio.Listener{OnSpeech: func() { setStatus("Hearing you…") }}
			wav, err := l.Listen(ctx)
			if err != nil {
				setStatus(err.Error())
				return
			}
			if wav == nil {
				setStatus("Nothing heard")
				return
			}
			setStatus("Playing it back")
			if err := audio.Play(ctx, wav, nil); err != nil {
				setStatus(err.Error())
				return
			}
			setStatus("Tap and speak")
		}()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Speak", OnPress: listen},
		ui.Text(status, ui.Style{}))
}
