package bridge

import (
	"bytes"
	"os"
	"testing"
)

// The iOS Swift package carries its own copy of nn.h: Go module zips leave
// symlinks out, so it can't point here.
func TestPackageHeaderMatches(t *testing.T) {
	want, err := os.ReadFile("include/nn.h")
	if err != nil {
		t.Fatal(err)
	}
	have, err := os.ReadFile("../hosts/ios/Sources/NimbusNativeC/include/nn.h")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(have, want) {
		t.Fatal("hosts/ios/Sources/NimbusNativeC/include/nn.h differs from bridge/include/nn.h: copy it over")
	}
}
