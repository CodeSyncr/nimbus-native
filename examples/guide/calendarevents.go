package guide

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/calendarevents"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Agenda books a table, and shows the week ahead.
type Agenda struct{}

func loadWeek(ctx context.Context) ([]calendarevents.Event, error) {
	ok, err := calendarevents.Request(ctx)
	if err != nil || !ok {
		return nil, err
	}
	return calendarevents.List(ctx, time.Now(), time.Now().AddDate(0, 0, 7))
}

func bookTable(ctx context.Context, _ struct{}) (string, error) {
	_, err := calendarevents.Create(ctx, calendarevents.Event{
		Title: "Table for two", Location: "Monsoon Coffee", Start: time.Now().Add(24 * time.Hour),
	})
	return "Booked", err
}

func (Agenda) Render(c *ui.Ctx) ui.Component {
	week := ui.UseQuery(c, "week", loadWeek)
	book := ui.UseAction(c, bookTable, func(string) { ui.RefetchQuery(c, "week") })
	rows := []ui.Component{ui.Button{Label: "Book a table", OnPress: func() { book.Run(struct{}{}) }}}
	if book.Err != nil {
		rows = append(rows, ui.Text(book.Err.Error(), ui.Style{}))
	}
	if week.Err != nil {
		rows = append(rows, ui.Text(week.Err.Error(), ui.Style{}))
	}
	for _, e := range week.Data {
		rows = append(rows, ui.ListRow{Title: e.Title, Subtitle: e.Start.Format("Mon 2 Jan, 3:04 pm"), Icon: "calendar"})
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12}, rows...)
}
