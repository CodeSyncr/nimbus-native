package components

import (
	"strings"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Menu filters as they type.
type Menu struct{}

func (Menu) Render(c *ui.Ctx) ui.Component {
	items := []string{"Flat white", "Cold brew", "Iced latte", "Banana bread"}
	query, setQuery := ui.State(c, "")
	rows := []ui.Component{ui.SearchBar{Value: query, Placeholder: "Search the menu", OnChange: setQuery}}
	for _, item := range items {
		if strings.Contains(strings.ToLower(item), strings.ToLower(query)) {
			rows = append(rows, ui.Text(item, ui.Style{}))
		}
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12}, rows...)
}
