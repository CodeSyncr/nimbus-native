//go:build !android

package bridge

import (
	"fmt"
	"os"
)

// Logf writes a line to the platform log: stderr here (the Xcode and
// simulator consoles show it), logcat on Android.
func Logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[nimbus-native] "+format+"\n", args...)
}
