package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Verify checks the code as soon as it's complete.
type Verify struct{}

func (Verify) Render(c *ui.Ctx) ui.Component {
	code, setCode := ui.State(c, "")
	line := "Enter the 6 digits we sent"
	if len(code) == 6 {
		line = "Checking " + code
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.OTPInput{Value: code, OnChange: setCode},
		ui.Text(line, ui.Style{}))
}
