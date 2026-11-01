package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/speech"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// ReadAloud reads a recipe step, hands free.
type ReadAloud struct{}

func (ReadAloud) Render(c *ui.Ctx) ui.Component {
	say := ui.Ref(c, func() {})
	stop := ui.Ref(c, func() {})
	*say = func() {
		ctx, cancel := context.WithCancel(context.Background())
		(*stop)()
		*stop = cancel
		go speech.Say(ctx, "Pour slowly in circles for thirty seconds.", speech.Options{Language: "en-IN"})
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Read next step", OnPress: func() { (*say)() }},
		ui.Button{Label: "Stop", Variant: ui.Tonal, OnPress: func() { (*stop)() }})
}
