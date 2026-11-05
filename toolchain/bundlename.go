package toolchain

import (
	"regexp"
	"strings"
)

// iOS takes an app's CFBundleName from PRODUCT_NAME, and shows it where the
// display name doesn't reach, such as "Runner wants to use … to sign in".
// So the app target's PRODUCT_NAME is the app's name, while the bundle,
// executable and Swift module keep Runner: Go's runtime chdirs into the
// bundle at launch and fails on a space in its path.

// NameIOSBundle gives a project from before this its app's name on iOS.
func NameIOSBundle(root string) error {
	return edit(root, projectXcode, nameBundle)
}

var buildConfig = regexp.MustCompile(`(?s)\{\n\t+isa = XCBuildConfiguration;.*?\n\t+name = [^;]+;\n\t+\}`)

// nameBundle sets PRODUCT_NAME to the display name in the app target's
// configurations (those with Runner/Info.plist) that still use the target's.
func nameBundle(pbx string) string {
	return buildConfig.ReplaceAllStringFunc(pbx, func(c string) string {
		const old = "\t\t\t\tPRODUCT_NAME = \"$(TARGET_NAME)\";\n"
		if !strings.Contains(c, "INFOPLIST_FILE = Runner/Info.plist;") || !strings.Contains(c, old) {
			return c
		}
		m := displayName.FindStringSubmatch(c)
		if m == nil {
			return c
		}
		c = strings.Replace(c, old, "\t\t\t\tPRODUCT_MODULE_NAME = Runner;\n\t\t\t\tPRODUCT_NAME = "+m[1]+";\n", 1)
		c = strings.Replace(c, "\t\t\t\tGENERATE_INFOPLIST_FILE", "\t\t\t\tEXECUTABLE_NAME = Runner;\n\t\t\t\tGENERATE_INFOPLIST_FILE", 1)
		i := strings.LastIndex(c, ";\n\t\t\t};") + 2 // after the last setting
		return c[:i] + "\t\t\t\tWRAPPER_NAME = Runner.app;\n" + c[i:]
	})
}
