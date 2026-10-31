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
	if s := enabledSwift(h, nil, []string{"audio"}); !strings.Contains(s, `Plugins.register("audio", Audio.handle)`) {
		t.Fatal(s)
	}
	if s := enabledKotlin(h, nil, []string{"audio"}); !strings.Contains(s, `Plugins.register("audio", Plugins.Handler(Audio::handle))`) {
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

func TestAndroidApplicationAndGradle(t *testing.T) {
	man := "<manifest>\n    <application>\n        <activity />\n    </application>\n</manifest>\n"
	x := `<receiver android:name="a.B" />`
	s := applicationAdd(man, x)
	if !strings.Contains(s, "<activity />\n        "+x+"\n    </application>") || applicationAdd(s, x) != s {
		t.Fatalf("application:\n%s", s)
	}
	if got := strings.Replace(s, "        "+x+"\n", "", 1); got != man {
		t.Fatalf("not removed:\n%s", got)
	}
	g := "android {\n}\n"
	s = gradleAdd(g, "a:b:1")
	if !strings.Contains(s, `implementation("a:b:1")`) || gradleAdd(s, "a:b:1") != s {
		t.Fatalf("gradle:\n%s", s)
	}
	if got := strings.TrimRight(gradleDrop(s, "a:b:1"), "\n"); got != strings.TrimRight(g, "\n") {
		t.Fatalf("not removed:\n%q", got)
	}
}

func TestPlistArraysAndEntitlements(t *testing.T) {
	in := "<plist>\n<dict>\n</dict>\n</plist>\n"
	s := plistAppend(in, "UIBackgroundModes", []string{"remote-notification"})
	if !strings.Contains(s, "<array>") || plistAppend(s, "UIBackgroundModes", []string{"remote-notification"}) != s {
		t.Fatalf("append:\n%s", s)
	}
	if got := plistRemove(s, "UIBackgroundModes", []string{"remote-notification"}); got != in {
		t.Fatalf("remove:\n%q", got)
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "native/ios/Runner"), 0o755)
	os.MkdirAll(filepath.Join(root, "native/ios/Runner.xcodeproj"), 0o755)
	xcode := filepath.Join(root, projectXcode)
	os.WriteFile(xcode, []byte("\t\t\t\tCODE_SIGN_STYLE = Automatic;\n"), 0o644)
	if err := writeEntitlements(root, map[string]string{"aps-environment": "development"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(xcode)
	e, _ := os.ReadFile(filepath.Join(root, projectEntitlements))
	if !bytes.Contains(b, []byte(entitlementsSetting)) || !bytes.Contains(e, []byte("aps-environment")) {
		t.Fatalf("%s\n%s", b, e)
	}
	if err := dropEntitlements(root, map[string]string{"aps-environment": ""}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(xcode)
	if bytes.Contains(b, []byte(entitlementsSetting)) || exists(filepath.Join(root, projectEntitlements)) {
		t.Fatalf("not removed: %s", b)
	}
}

func TestKeysFillIn(t *testing.T) {
	m := &pluginManifest{}
	m.IOS.Info = map[string]string{"GMSApiKey": "{{key}}"}
	m.Android.Application = []string{`<meta-data android:value="{{key}}" />`}
	f := fill(m, map[string]string{"key": "abc"})
	if f.IOS.Info["GMSApiKey"] != "abc" || f.Android.Application[0] != `<meta-data android:value="abc" />` {
		t.Fatalf("%+v", f)
	}
	man := "<application>\n        <meta-data android:value=\"abc\" />\n    </application>\n"
	if got := applicationDrop(man, m.Android.Application[0]); strings.Contains(got, "meta-data") {
		t.Fatalf("not removed:\n%s", got)
	}
}

func TestPushConfig(t *testing.T) {
	root := t.TempDir()
	if k, err := pushConfigKotlin(root, "a.b"); err != nil || !strings.Contains(k, `APP_ID = ""`) {
		t.Fatalf("no file yet: %v\n%s", err, k)
	}
	os.MkdirAll(filepath.Join(root, "native/android/app"), 0o755)
	body := `{"project_info":{"project_number":"123","project_id":"cafe"},"client":[
		{"client_info":{"mobilesdk_app_id":"1:123:android:x","android_client_info":{"package_name":"a.b"}},"api_key":[{"current_key":"KEY"}]}]}`
	os.WriteFile(filepath.Join(root, googleServices), []byte(body), 0o644)
	k, err := pushConfigKotlin(root, "a.b")
	if err != nil || !strings.Contains(k, `"1:123:android:x"`) || !strings.Contains(k, `"KEY"`) || !strings.Contains(k, `SENDER = "123"`) {
		t.Fatalf("%v\n%s", err, k)
	}
	if _, err := pushConfigKotlin(root, "other.app"); err == nil {
		t.Fatal("a file for another app was accepted")
	}
}
