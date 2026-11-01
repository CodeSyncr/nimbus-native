package guide

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/backgroundtasks"
	"github.com/CodeSyncr/nimbus-native/plugins/filesystem"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// init runs when the system starts the app with no screen, so the task has
// to be registered here, not in a component.
func init() {
	backgroundtasks.Handle("sync", func(ctx context.Context) error {
		stamp := time.Now().Format(time.RFC3339)
		return filesystem.Write(ctx, filesystem.Data, "last-sync.txt", []byte(stamp))
	})
}

// NightlySync asks for the task, and shows when it last ran.
type NightlySync struct{}

func loadLastSync(ctx context.Context) (string, error) {
	if ok, err := filesystem.Exists(ctx, filesystem.Data, "last-sync.txt"); err != nil || !ok {
		return "Not yet", err
	}
	b, err := filesystem.Read(ctx, filesystem.Data, "last-sync.txt")
	return string(b), err
}

func (NightlySync) Render(c *ui.Ctx) ui.Component {
	last := ui.UseQuery(c, "last-sync", loadLastSync)
	ui.Effect(c, func(ctx context.Context) func() {
		go backgroundtasks.Schedule(ctx, "sync", 6*time.Hour)
		return nil
	}, ui.Once)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 8},
		ui.Text("Last sync", ui.Style{FontWeight: 600}),
		ui.Text(last.Data, ui.Style{}))
}
