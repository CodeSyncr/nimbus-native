package bridge

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestABI builds internal/abitest/app as a C archive, links it with the C
// host in internal/abitest/host.c, and runs the host: the full nn.h
// contract checked on the desktop, no simulator needed.
func TestABI(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a C archive; skipped with -short")
	}
	cc := goEnv(t, "CC")
	if _, err := exec.LookPath(strings.Fields(cc)[0]); err != nil {
		t.Skipf("no C compiler (%s)", cc)
	}
	dir := t.TempDir()
	lib := filepath.Join(dir, "libapp.a")

	build := exec.Command("go", "build", "-buildmode=c-archive", "-o", lib, "./internal/abitest/app")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	bin := filepath.Join(dir, "host")
	args := append(strings.Fields(cc)[1:], "-O2", "-Iinclude", "internal/abitest/host.c", lib, "-o", bin, "-lpthread")
	if runtime.GOOS == "darwin" {
		args = append(args, "-framework", "CoreFoundation", "-lresolv")
	}
	link := exec.Command(strings.Fields(cc)[0], args...)
	if out, err := link.CombinedOutput(); err != nil {
		t.Fatalf("link host: %v\n%s", err, out)
	}

	run := exec.Command(bin)
	out, err := run.CombinedOutput()
	t.Logf("\n%s", out)
	if err != nil || !strings.Contains(string(out), "PASS") {
		t.Fatalf("host failed: %v", err)
	}
}

func goEnv(t *testing.T, key string) string {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		t.Fatalf("go env %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}
