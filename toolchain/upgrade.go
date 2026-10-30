package toolchain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Upgrade moves a project to another nimbus-native release: version is
// "latest" or a tag like v0.0.10. go.mod changes, then the new release's
// own toolchain syncs the project's copies (nimbus native sync), so the
// kits and generated code come from the version the app now builds with.
// It returns the versions before and after.
func (e *Env) Upgrade(root, version string) (from, to string, err error) {
	if version == "" {
		version = "latest"
	}
	if from, err = requiredVersion(root); err != nil {
		return "", "", err
	}
	if b, _ := os.ReadFile(filepath.Join(root, "go.mod")); strings.Contains(string(b), "replace "+nativeModule) {
		return "", "", fmt.Errorf("go.mod replaces %s with a local copy: remove the replace line to upgrade", nativeModule)
	}
	e.step("go get %s@%s", nativeModule, version)
	// GOPROXY lags a new tag by a few minutes; asking the source is exact.
	if err := e.runIn(root, []string{"GOPROXY=direct"}, "go", "get", nativeModule+"@"+version); err != nil {
		return "", "", err
	}
	e.step("go mod tidy")
	if err := e.runIn(root, nil, "go", "mod", "tidy"); err != nil {
		return "", "", err
	}
	if to, err = requiredVersion(root); err != nil {
		return "", "", err
	}
	e.step("nimbus native sync (%s)", to)
	cmd := exec.Command("go", "run", nativeModule+"/cmd/nimbus-native", "sync")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.HasPrefix(string(out), "usage:") {
			// A release older than sync: its builds sync the project themselves.
			fmt.Fprintf(e.Out, "  (%s syncs native/ on its first build: run it once more if asked)\n", to)
			return from, to, nil
		}
		return from, to, fmt.Errorf("go.mod now requires %s, but syncing the project failed: %v\n%s"+
			"(the next build syncs it too)", to, err, out)
	}
	return from, to, nil
}

const nativeModule = "github.com/CodeSyncr/nimbus-native"

func requiredVersion(root string) (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}", nativeModule)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s doesn't require %s: is it a Nimbus Native project?", root, nativeModule)
	}
	return strings.TrimSpace(string(out)), nil
}
