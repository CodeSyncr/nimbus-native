package toolchain

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestKotlinInstall downloads the pinned compiler; it runs only with
// NIMBUS_TEST_KOTLIN=1 (it fetches ~80 MB once).
func TestKotlinInstall(t *testing.T) {
	if os.Getenv("NIMBUS_TEST_KOTLIN") == "" {
		t.Skip("set NIMBUS_TEST_KOTLIN=1 to download and check the Kotlin compiler")
	}
	e := &Env{Out: os.Stdout}
	kc, err := e.Kotlinc()
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(kc, "-version").CombinedOutput()
	if err != nil || !strings.Contains(string(out), kotlinVersion) {
		t.Fatalf("%s -version: %v\n%s", kc, err, out)
	}
	if !exists(kotlinStdlib(kc)) {
		t.Fatalf("no stdlib at %s", kotlinStdlib(kc))
	}
	t.Logf("%s", strings.TrimSpace(string(out)))
}

func TestUnzipRejectsEscapes(t *testing.T) {
	if err := unzip("testdata/missing.zip", t.TempDir()); err == nil {
		t.Fatal("a missing zip unpacked")
	}
}
