package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/audioplayer"
	"github.com/CodeSyncr/nimbus-native/plugins/audiorecorder"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// VoiceMemo records a note, shows the level, and plays it back.
type VoiceMemo struct{}

func (VoiceMemo) Render(c *ui.Ctx) ui.Component {
	live, setLive := ui.State(c, audiorecorder.Status{})
	memo, setMemo := ui.State(c, audiorecorder.Recording{})
	status, setStatus := ui.State(c, "")
	stop := ui.Ref(c, func() {})
	ctx := context.Background()
	start := func() {
		go func() {
			ok, err := audiorecorder.Request(ctx)
			if err != nil || !ok {
				setStatus("The microphone is off")
				return
			}
			if err := audiorecorder.Start(ctx); err != nil {
				setStatus(err.Error())
				return
			}
			setStatus("Recording")
			watch, cancel := context.WithCancel(ctx)
			*stop = cancel
			audiorecorder.Watch(watch, setLive)
		}()
	}
	finish := func() {
		go func() {
			(*stop)()
			r, err := audiorecorder.Stop(ctx)
			if err != nil {
				setStatus(err.Error())
				return
			}
			setMemo(r)
			setStatus(fmt.Sprintf("%.1f seconds, %d KB", r.Seconds, len(r.Data)/1024))
		}()
	}
	listen := func() {
		go func() {
			p, err := audioplayer.Open(ctx, audioplayer.Source{Data: memo.Data, Type: memo.Type})
			if err != nil {
				setStatus(err.Error())
				return
			}
			p.Play(ctx)
			setStatus("Playing it back")
		}()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.ProgressBar{Value: live.Level},
		ui.Text(fmt.Sprintf("%.1f seconds", live.Seconds), ui.Style{FontSize: 28}),
		ui.Button{Label: "Record", OnPress: start},
		ui.Button{Label: "Stop", Variant: ui.Tonal, OnPress: finish},
		ui.Button{Label: "Listen", Variant: ui.Tonal, OnPress: listen},
		ui.Text(status, ui.Style{}))
}
