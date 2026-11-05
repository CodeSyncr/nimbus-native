package toolchain

import (
	"os"
	"strings"
	"testing"
)

// A project made before PRODUCT_NAME was the app's name comes out the same
// as one made now, and only the app target changes.
func TestNameBundle(t *testing.T) {
	now, err := os.ReadFile("template/common/native/ios/Runner.xcodeproj/project.pbxproj")
	if err != nil {
		t.Fatal(err)
	}
	before := strings.NewReplacer(
		"\t\t\t\tEXECUTABLE_NAME = Runner;\n", "",
		"\t\t\t\tPRODUCT_MODULE_NAME = Runner;\n", "",
		"\t\t\t\tWRAPPER_NAME = Runner.app;\n", "",
		`PRODUCT_NAME = "__NAME__";`, `PRODUCT_NAME = "$(TARGET_NAME)";`,
	).Replace(string(now))
	got := nameBundle(before)
	if got != string(now) {
		t.Fatalf("an old project doesn't come out like a new one:\n%s", got)
	}
	if nameBundle(got) != got {
		t.Fatal("a second run changed it again")
	}
	other := strings.ReplaceAll(before, "INFOPLIST_FILE = Runner/Info.plist;", "INFOPLIST_FILE = ShareExtension/Info.plist;")
	if nameBundle(other) != other {
		t.Fatal("changed a target that isn't the app")
	}
}
