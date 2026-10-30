package toolchain

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistEdits(t *testing.T) {
	in := "<plist>\n<dict>\n\t<key>A</key>\n\t<string>a</string>\n</dict>\n</plist>\n"
	s := plistSet(in, "NSCameraUsageDescription", "Scans <codes> & more")
	if !strings.Contains(s, "<key>NSCameraUsageDescription</key>\n\t<string>Scans &lt;codes&gt; &amp; more</string>") {
		t.Fatalf("not added:\n%s", s)
	}
	if plistSet(s, "NSCameraUsageDescription", "other") != s {
		t.Fatal("overwrote the app's own wording")
	}
	if got := plistDrop(s, "NSCameraUsageDescription"); got != in {
		t.Fatalf("not removed:\n%s", got)
	}
}

func TestManifestEdits(t *testing.T) {
	in := "<manifest xmlns:android=\"x\">\n\n    <uses-permission android:name=\"android.permission.INTERNET\" />\n\n    <application />\n</manifest>\n"
	s := manifestAdd(in, "android.permission.CAMERA")
	if !strings.Contains(s, "INTERNET\" />\n    <uses-permission android:name=\"android.permission.CAMERA\" />\n") {
		t.Fatalf("not added:\n%s", s)
	}
	if manifestAdd(s, "android.permission.CAMERA") != s {
		t.Fatal("added twice")
	}
	if got := manifestDrop(s, "android.permission.CAMERA"); got != in {
		t.Fatalf("not removed:\n%s", got)
	}
	bare := "<manifest xmlns:android=\"x\">\n    <application />\n</manifest>\n"
	if s := manifestAdd(bare, "android.permission.CAMERA"); !strings.Contains(s, "uses-permission") || strings.Index(s, "uses-permission") > strings.Index(s, "<application") {
		t.Fatalf("no permissions yet:\n%s", s)
	}
}

func TestEnabledPluginCode(t *testing.T) {
	h := map[string]string{"audio": "Audio"}
	if s := enabledSwift(h, []string{"audio"}); !strings.Contains(s, `Plugins.register("audio", Audio.handle)`) {
		t.Fatal(s)
	}
	if s := enabledKotlin(h, []string{"audio"}); !strings.Contains(s, `Plugins.register("audio", Plugins.Handler(Audio::handle))`) {
		t.Fatal(s)
	}
}

// Every plugin in the repo has what a project needs from it.
func TestPluginManifests(t *testing.T) {
	e := &Env{Repo: ".."}
	paths := globs("../plugins/*/plugin.json")
	if len(paths) == 0 {
		t.Fatal("no plugins")
	}
	for _, p := range paths {
		name := filepath.Base(filepath.Dir(p))
		m, err := e.pluginManifest(name)
		if err != nil {
			t.Fatal(err)
		}
		if m.Handler == "" || m.Description == "" {
			t.Errorf("%s: no handler or description", name)
		}
		if len(globs("../plugins/"+name+"/ios/*.swift")) == 0 || len(globs("../plugins/"+name+"/android/*.kt")) == 0 {
			t.Errorf("%s: sources missing", name)
		}
	}
}

func TestAddAndRemoveInAProject(t *testing.T) {
	root := t.TempDir()
	for p, body := range map[string]string{
		projectPlist:    "<plist>\n<dict>\n</dict>\n</plist>\n",
		projectManifest: "<manifest>\n    <uses-permission android:name=\"android.permission.INTERNET\" />\n</manifest>\n",
	} {
		os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte(body), 0o644)
	}
	e := &Env{Repo: ".."}
	m, err := e.pluginManifest("audio")
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePlugin(root, m); err != nil {
		t.Fatal(err)
	}
	plist, _ := os.ReadFile(filepath.Join(root, projectPlist))
	man, _ := os.ReadFile(filepath.Join(root, projectManifest))
	if !bytes.Contains(plist, []byte("NSMicrophoneUsageDescription")) || !bytes.Contains(man, []byte("RECORD_AUDIO")) {
		t.Fatalf("%s\n%s", plist, man)
	}
	if err := dropPlugin(root, m, map[string]bool{}, map[string]bool{"android.permission.MODIFY_AUDIO_SETTINGS": true}); err != nil {
		t.Fatal(err)
	}
	plist, _ = os.ReadFile(filepath.Join(root, projectPlist))
	man, _ = os.ReadFile(filepath.Join(root, projectManifest))
	if bytes.Contains(plist, []byte("NSMicrophone")) || bytes.Contains(man, []byte("RECORD_AUDIO")) || !bytes.Contains(man, []byte("MODIFY_AUDIO")) {
		t.Fatalf("%s\n%s", plist, man)
	}
	if err := writePluginList(root, []string{"audio"}); err != nil {
		t.Fatal(err)
	}
	got, err := enabledPlugins(root)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	var check []string
	b, _ := os.ReadFile(filepath.Join(root, pluginList))
	if json.Unmarshal(b, &check) != nil {
		t.Fatalf("not JSON: %s", b)
	}
}
