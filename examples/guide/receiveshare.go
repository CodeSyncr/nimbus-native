package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/receiveshare"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Inbox lists what people shared to the app from elsewhere: a link to read
// later, a photo of a receipt.
type Inbox struct{}

func (Inbox) Render(c *ui.Ctx) ui.Component {
	got, setGot := ui.State(c, []receiveshare.Share{})
	list := ui.Ref(c, []receiveshare.Share{})
	ui.Effect(c, func(ctx context.Context) func() {
		go receiveshare.Watch(ctx, receiveshare.Options{}, func(s receiveshare.Share) {
			*list = append([]receiveshare.Share{s}, *list...)
			setGot(*list)
		})
		return nil
	}, ui.Once)
	var rows []ui.Component
	for _, s := range got {
		title := s.Text
		if s.URL != "" {
			title = s.URL
		}
		if len(s.Files) > 0 {
			title = fmt.Sprintf("%s (%d KB)", s.Files[0].Name, s.Files[0].Size/1024)
		}
		rows = append(rows, ui.ListRow{Title: title, Subtitle: s.At.Format("Jan 2, 3:04 pm")})
	}
	if len(rows) == 0 {
		rows = append(rows, ui.Text("Share something to this app from Photos or Safari.", ui.Style{}))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12}, rows...)
}
