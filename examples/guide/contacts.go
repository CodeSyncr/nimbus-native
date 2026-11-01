package guide

import (
	"context"
	"strings"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/contacts"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// SendGift lets them choose one person. The system's picker needs no
// permission: the app only sees who they chose.
type SendGift struct{}

func choosePerson(ctx context.Context, _ struct{}) (string, error) {
	p, err := contacts.Pick(ctx)
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	return p.Name + " " + strings.Join(p.Phones, ", "), err
}

func (SendGift) Render(c *ui.Ctx) ui.Component {
	pick := ui.UseAction(c, choosePerson)
	status := pick.Result
	if pick.Err != nil {
		status = pick.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Choose from contacts", OnPress: func() { pick.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
