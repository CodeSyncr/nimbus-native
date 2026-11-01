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

func saveCafe(ctx context.Context, _ struct{}) (string, error) {
	saved, err := contacts.Add(ctx, contacts.Contact{Name: "Monsoon Coffee", Phones: []string{"+912012345678"}})
	if err != nil || !saved {
		return "", err
	}
	return "Check your contacts", nil // Android can't say whether they saved it
}

func (SendGift) Render(c *ui.Ctx) ui.Component {
	pick, save := ui.UseAction(c, choosePerson), ui.UseAction(c, saveCafe)
	status := pick.Result + save.Result
	if pick.Err != nil {
		status = pick.Err.Error()
	}
	if save.Err != nil {
		status = save.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Choose from contacts", OnPress: func() { pick.Run(struct{}{}) }},
		ui.Button{Label: "Save the cafe", Variant: ui.Tonal, OnPress: func() { save.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
