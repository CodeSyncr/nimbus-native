// Package backgroundtasks runs Go now and then, with the app closed.
//
//	nimbus native plugin add background-tasks
//
//	func init() {
//		backgroundtasks.Handle("sync", func(ctx context.Context) error {
//			return uploadOrders(ctx)
//		})
//	}
//
//	backgroundtasks.Schedule(ctx, "sync", time.Hour)
//
// The system picks the moment, from how the phone is used, so treat the
// interval as a hint. Android runs a task at most every 15 minutes, and
// iOS far less often, and never after the person force-quits the app. A task
// has about 25 seconds. There is no screen: ui code doesn't run.
package backgroundtasks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// limit is how long a task gets: iOS allows about 30 seconds.
const limit = 25 * time.Second

var (
	mu       sync.Mutex
	handlers = map[string]func(context.Context) error{}
)

func init() { plugin.OnBackground = run }

// Handle says what a task does. Call it from init, so that it's there when
// the system starts the app with no screen.
func Handle(name string, f func(context.Context) error) {
	mu.Lock()
	defer mu.Unlock()
	handlers[name] = f
}

func run(name string) {
	mu.Lock()
	f := handlers[name]
	mu.Unlock()
	ok := false
	if f != nil {
		ctx, cancel := context.WithTimeout(context.Background(), limit)
		defer cancel()
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("background task %s: panic: %v\n", name, r)
				}
			}()
			ok = f(ctx) == nil
		}()
	}
	done, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = plugin.CallJSON(done, "backgroundtasks", "done", map[string]any{"name": name, "ok": ok})
}

// Schedule asks for a task to run about every so often. Scheduling it again
// replaces the interval.
func Schedule(ctx context.Context, name string, every time.Duration) error {
	_, err := plugin.CallJSON(ctx, "backgroundtasks", "schedule", map[string]any{"name": name, "seconds": every.Seconds()})
	return err
}

// Once asks for a task to run one time, no sooner than after: to retry an
// upload later, say.
func Once(ctx context.Context, name string, after time.Duration) error {
	_, err := plugin.CallJSON(ctx, "backgroundtasks", "once", map[string]any{"name": name, "seconds": after.Seconds()})
	return err
}

// Cancel stops a task.
func Cancel(ctx context.Context, name string) error {
	_, err := plugin.CallJSON(ctx, "backgroundtasks", "cancel", map[string]string{"name": name})
	return err
}
