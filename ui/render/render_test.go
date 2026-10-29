package render_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/CodeSyncr/nimbus-native/wire/gen"
)

var update = flag.Bool("update", false, "rewrite the generated Go, Swift and Java")

// The protocol's generated code is committed in three places: here, in the
// iOS kit and in the Android kit (Kotlin). They must all match the schema.
func TestGeneratedUpToDate(t *testing.T) {
	out, err := gen.Generate(".", "space.nimbusgo.nativekit")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		gen.GoFile: out.Go,
		"../../hosts/ios/Sources/NimbusNativeKit/Render.swift":       out.Swift,
		"../../hosts/android/src/space/nimbusgo/nativekit/Render.kt": out.Kotlin,
	}
	for path, want := range files {
		if *update {
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		have, _ := os.ReadFile(path)
		if !bytes.Equal(have, want) {
			t.Errorf("%s is stale; run go test ./ui/render -update", filepath.Clean(path))
		}
	}
}
