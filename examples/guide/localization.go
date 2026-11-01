package guide

import (
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/localization"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Prices shows a price and a distance the way they're used to.
type Prices struct{}

func (Prices) Render(c *ui.Ctx) ui.Component {
	q := ui.UseQuery(c, "locale", localization.Get)
	if q.Err != nil {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text(q.Err.Error(), ui.Style{}))
	}
	l := q.Data
	distance := "1.6 km"
	if !l.Metric {
		distance = "1 mile"
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 8},
		ui.Text(l.CurrencySymbol+"220"+l.DecimalSeparator+"00", ui.Style{FontSize: 28}),
		ui.Text(distance+" away", ui.Style{}),
		ui.Text(fmt.Sprintf("%v, %s %s", l.Languages, l.Region, l.TimeZone), ui.Style{}),
		ui.Text(fmt.Sprintf("24 hour: %v, week starts on day %d", l.Uses24Hour, l.FirstWeekday), ui.Style{}))
}
