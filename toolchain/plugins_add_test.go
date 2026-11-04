package toolchain

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A plugin that's missing a value stops the whole add before anything
// changes: the others used to get their permissions but never reach the list.
func TestAddChecksEveryPluginFirst(t *testing.T) {
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for p, body := range map[string]string{
		"go.mod":        "module cafe\n\ngo 1.23\n\nrequire github.com/CodeSyncr/nimbus-native v0.0.0\n\nreplace github.com/CodeSyncr/nimbus-native => " + repo + "\n",
		projectPlist:    "<plist>\n<dict>\n</dict>\n</plist>\n",
		projectManifest: "<manifest>\n    <uses-permission android:name=\"android.permission.INTERNET\" />\n</manifest>\n",
	} {
		os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte(body), 0o644)
	}
	e := &Env{}
	err = e.AddPlugins(&App{Dir: root, Root: root}, []string{"audio", "oauth"}, nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "-scheme") {
		t.Fatalf("want oauth's -scheme asked for, got %v", err)
	}
	plist, _ := os.ReadFile(filepath.Join(root, projectPlist))
	man, _ := os.ReadFile(filepath.Join(root, projectManifest))
	if strings.Contains(string(plist), "NSMicrophone") || strings.Contains(string(man), "RECORD_AUDIO") {
		t.Fatalf("audio was half added:\n%s\n%s", plist, man)
	}
	if _, err := os.Stat(filepath.Join(root, pluginList)); !os.IsNotExist(err) {
		t.Fatalf("%s was written: %v", pluginList, err)
	}
}
