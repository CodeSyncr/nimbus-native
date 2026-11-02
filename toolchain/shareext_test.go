package toolchain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func templatePbx(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("template/common/native/ios/Runner.xcodeproj/project.pbxproj")
	if err != nil {
		t.Fatal(err)
	}
	return strings.NewReplacer("__ID__", "space.nimbusgo.cafe", "__NAME__", "Cafe").Replace(string(b))
}

func TestShareExtensionTarget(t *testing.T) {
	in := templatePbx(t)
	s := pbxAddShareExtension(in, "space.nimbusgo.cafe")
	for _, want := range []string{
		"PRODUCT_BUNDLE_IDENTIFIER = space.nimbusgo.cafe.ShareExtension;",
		`INFOPLIST_KEY_CFBundleDisplayName = "Cafe";`,
		"productType = \"com.apple.product-type.app-extension\";",
		"\t\t\t\tAA0000000000000000000072 /* Resources */,\n\t\t\t\t" + extEmbedPhase + " /* Embed Foundation Extensions */,\n",
		"\t\t\t\tAA0000000000000000000060 /* Runner */,\n\t\t\t\t" + extTarget + " /* ShareExtension */,\n",
		"\t\t\t\tAA0000000000000000000020 /* Runner.app */,\n\t\t\t\t" + extAppex + " " + shareExtMarker + ",\n",
		"\t\t\t\tAA0000000000000000000051 /* Products */,\n\t\t\t\t" + extGroup + " /* ShareExtension */,\n",
		"\t\t\tdependencies = (\n\t\t\t\t" + extDependency + " /* PBXTargetDependency */,\n",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	// Every object the extension refers to is there.
	for _, id := range extIDs {
		if !strings.Contains(s, "\t\t"+id+" ") {
			t.Errorf("no object %s", id)
		}
	}
	if pbxAddShareExtension(s, "space.nimbusgo.cafe") != s {
		t.Error("added twice")
	}
	if got := pbxDropShareExtension(s); got != in {
		t.Errorf("not removed cleanly:\n%s", got)
	}
}

// Xcode may add attributes for the target when it saves; those go too.
func TestShareExtensionDropAfterXcode(t *testing.T) {
	in := templatePbx(t)
	s := pbxAddShareExtension(in, "space.nimbusgo.cafe")
	s = strings.Replace(s, "\t\t\t\t\tAA0000000000000000000060 = {\n", "\t\t\t\t\t"+extTarget+" = {\n\t\t\t\t\t\tCreatedOnToolsVersion = 16.2;\n\t\t\t\t\t};\n\t\t\t\t\tAA0000000000000000000060 = {\n", 1)
	got := pbxDropShareExtension(s)
	if got != in {
		t.Errorf("left behind:\n%s", got)
	}
}

func TestAddShareExtensionFiles(t *testing.T) {
	root := t.TempDir()
	xc := filepath.Join(root, projectXcode)
	os.MkdirAll(filepath.Dir(xc), 0o755)
	os.WriteFile(xc, []byte(templatePbx(t)), 0o644)
	e := &Env{Repo: ".."}
	if err := e.addShareExtension(root, "receiveshare"); err != nil {
		t.Fatal(err)
	}
	ent, err := os.ReadFile(filepath.Join(root, shareExtDir, "ShareExtension.entitlements"))
	if err != nil || !strings.Contains(string(ent), "<string>group.space.nimbusgo.cafe</string>") {
		t.Fatalf("entitlements: %s %v", ent, err)
	}
	for _, f := range []string{"ShareViewController.swift", "Info.plist"} {
		if !exists(filepath.Join(root, shareExtDir, f)) {
			t.Errorf("no %s", f)
		}
	}
	if err := dropShareExtension(root); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(xc)
	if exists(filepath.Join(root, shareExtDir)) || string(b) != templatePbx(t) {
		t.Error("not removed")
	}
}
