package toolchain

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A Share Extension is how an iOS app joins the share sheet: a second
// target in the Xcode project, which copies what was shared into an App
// Group the app reads. receive-share asks for it in plugin.json
// ("shareExtension"). It's pure Swift and doesn't link Go.

const (
	shareExtDir    = "native/ios/ShareExtension"
	shareExtMarker = "/* ShareExtension.appex */"
)

// shareExtID is one of the extension's objects in project.pbxproj: the
// template's IDs are AA + 20 hex digits up to 0x92, so these can't collide.
func shareExtID(n int) string { return fmt.Sprintf("AA%020X", 0x500+n) }

var (
	extAppex      = shareExtID(1)
	extEmbedFile  = shareExtID(2)
	extEmbedPhase = shareExtID(3)
	extGroup      = shareExtID(4)
	extExceptions = shareExtID(5)
	extTarget     = shareExtID(10)
	extSources    = shareExtID(11)
	extFrameworks = shareExtID(12)
	extResources  = shareExtID(13)
	extProxy      = shareExtID(14)
	extDependency = shareExtID(15)
	extConfigs    = shareExtID(20)
	extDebug      = shareExtID(21)
	extRelease    = shareExtID(22)
	extIDs        = []string{extAppex, extEmbedFile, extEmbedPhase, extGroup, extExceptions, extTarget, extSources,
		extFrameworks, extResources, extProxy, extDependency, extConfigs, extDebug, extRelease}
)

// addShareExtension writes the extension's files from the plugin's
// extension/ folder, and adds its target once. Adding the plugin again
// brings the files up to date.
func (e *Env) addShareExtension(root, plugin string) error {
	if !exists(filepath.Join(root, projectXcode)) {
		return nil // no iOS app in this project
	}
	id, err := IOSProjectID(root)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, shareExtDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"ShareViewController.swift", "Info.plist"} {
		b, err := os.ReadFile(filepath.Join(e.Repo, "plugins", plugin, "extension", name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return err
		}
	}
	ent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>com.apple.security.application-groups</key>
	<array>
		<string>group.%s</string>
	</array>
</dict>
</plist>
`, xmlEscape(id))
	if err := os.WriteFile(filepath.Join(dir, "ShareExtension.entitlements"), []byte(ent), 0o644); err != nil {
		return err
	}
	return edit(root, projectXcode, func(s string) string { return pbxAddShareExtension(s, id) })
}

// dropShareExtension takes the target and its files away.
func dropShareExtension(root string) error {
	if err := os.RemoveAll(filepath.Join(root, shareExtDir)); err != nil {
		return err
	}
	return edit(root, projectXcode, pbxDropShareExtension)
}

var (
	displayName    = regexp.MustCompile(`INFOPLIST_KEY_CFBundleDisplayName = ("[^"]*"|[^;]+);`)
	marketingVer   = regexp.MustCompile(`MARKETING_VERSION = ([^;]+);`)
	projectVersion = regexp.MustCompile(`CURRENT_PROJECT_VERSION = ([^;]+);`)
)

func firstMatch(re *regexp.Regexp, s, fallback string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return fallback
}

func pbxAddShareExtension(s, appID string) string {
	if strings.Contains(s, shareExtMarker) {
		return s
	}
	runner := pbxTargetID(s, "Runner")
	project := firstMatch(regexp.MustCompile(`rootObject = (\w+)`), s, "")
	if runner == "" || project == "" {
		return s // not a project this knows how to change
	}
	name := firstMatch(displayName, s, `"$(TARGET_NAME)"`)
	version, build := firstMatch(marketingVer, s, "0.1.0"), firstMatch(projectVersion, s, "1")
	config := func(id, kind string) string {
		return fmt.Sprintf(`		%s /* %s */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				APPLICATION_EXTENSION_API_ONLY = YES;
				CODE_SIGN_ENTITLEMENTS = ShareExtension/ShareExtension.entitlements;
				CODE_SIGN_STYLE = Automatic;
				CURRENT_PROJECT_VERSION = %s;
				GENERATE_INFOPLIST_FILE = YES;
				INFOPLIST_FILE = ShareExtension/Info.plist;
				INFOPLIST_KEY_CFBundleDisplayName = %s;
				LD_RUNPATH_SEARCH_PATHS = (
					"$(inherited)",
					"@executable_path/Frameworks",
					"@executable_path/../../Frameworks",
				);
				MARKETING_VERSION = %s;
				PRODUCT_BUNDLE_IDENTIFIER = %s.ShareExtension;
				PRODUCT_NAME = "$(TARGET_NAME)";
				SKIP_INSTALL = YES;
				SWIFT_VERSION = 5.0;
				TARGETED_DEVICE_FAMILY = "1,2";
			};
			name = %s;
		};
`, id, kind, build, name, version, appID, kind)
	}
	phase := func(id, isa, name string) string {
		return fmt.Sprintf("\t\t%s /* %s */ = {\n\t\t\tisa = %s;\n\t\t\tbuildActionMask = 2147483647;\n\t\t\tfiles = (\n\t\t\t);\n\t\t\trunOnlyForDeploymentPostprocessing = 0;\n\t\t};\n", id, name, isa)
	}
	objects := fmt.Sprintf("\t\t%s %s = {isa = PBXFileReference; explicitFileType = \"wrapper.app-extension\"; includeInIndex = 0; path = ShareExtension.appex; sourceTree = BUILT_PRODUCTS_DIR; };\n", extAppex, shareExtMarker) +
		fmt.Sprintf("\t\t%s /* ShareExtension.appex in Embed Foundation Extensions */ = {isa = PBXBuildFile; fileRef = %s %s; settings = {ATTRIBUTES = (RemoveHeadersOnCopy, ); }; };\n", extEmbedFile, extAppex, shareExtMarker) +
		fmt.Sprintf(`		%s /* Embed Foundation Extensions */ = {
			isa = PBXCopyFilesBuildPhase;
			buildActionMask = 2147483647;
			dstPath = "";
			dstSubfolderSpec = 13;
			files = (
				%s /* ShareExtension.appex in Embed Foundation Extensions */,
			);
			name = "Embed Foundation Extensions";
			runOnlyForDeploymentPostprocessing = 0;
		};
		%s /* ShareExtension */ = {
			isa = PBXFileSystemSynchronizedRootGroup;
			exceptions = (
				%s /* Exceptions for "ShareExtension" folder in "ShareExtension" target */,
			);
			path = ShareExtension;
			sourceTree = "<group>";
		};
		%s /* Exceptions for "ShareExtension" folder in "ShareExtension" target */ = {
			isa = PBXFileSystemSynchronizedBuildFileExceptionSet;
			membershipExceptions = (
				Info.plist,
			);
			target = %s /* ShareExtension */;
		};
		%s /* ShareExtension */ = {
			isa = PBXNativeTarget;
			buildConfigurationList = %s /* Build configuration list for PBXNativeTarget "ShareExtension" */;
			buildPhases = (
				%s /* Sources */,
				%s /* Frameworks */,
				%s /* Resources */,
			);
			buildRules = (
			);
			dependencies = (
			);
			fileSystemSynchronizedGroups = (
				%s /* ShareExtension */,
			);
			name = ShareExtension;
			productName = ShareExtension;
			productReference = %s %s;
			productType = "com.apple.product-type.app-extension";
		};
`, extEmbedPhase, extEmbedFile, extGroup, extExceptions, extExceptions, extTarget, extTarget, extConfigs,
			extSources, extFrameworks, extResources, extGroup, extAppex, shareExtMarker) +
		phase(extSources, "PBXSourcesBuildPhase", "Sources") +
		phase(extFrameworks, "PBXFrameworksBuildPhase", "Frameworks") +
		phase(extResources, "PBXResourcesBuildPhase", "Resources") +
		fmt.Sprintf(`		%s /* PBXContainerItemProxy */ = {
			isa = PBXContainerItemProxy;
			containerPortal = %s /* Project object */;
			proxyType = 1;
			remoteGlobalIDString = %s;
			remoteInfo = ShareExtension;
		};
		%s /* PBXTargetDependency */ = {
			isa = PBXTargetDependency;
			target = %s /* ShareExtension */;
			targetProxy = %s /* PBXContainerItemProxy */;
		};
		%s /* Build configuration list for PBXNativeTarget "ShareExtension" */ = {
			isa = XCConfigurationList;
			buildConfigurations = (
				%s /* Debug */,
				%s /* Release */,
			);
			defaultConfigurationIsVisible = 0;
			defaultConfigurationName = Release;
		};
`, extProxy, project, extTarget, extDependency, extTarget, extProxy, extConfigs, extDebug, extRelease) +
		config(extDebug, "Debug") + config(extRelease, "Release")

	end := strings.LastIndex(s, "\n\t};\n\trootObject")
	if end < 0 {
		return s
	}
	s = s[:end+1] + objects + s[end+1:]
	s = pbxListAdd(s, runner, "buildPhases", extEmbedPhase+" /* Embed Foundation Extensions */")
	s = pbxListAdd(s, runner, "dependencies", extDependency+" /* PBXTargetDependency */")
	s = pbxListAdd(s, project, "targets", extTarget+" /* ShareExtension */")
	if products := firstMatch(regexp.MustCompile(`productRefGroup = (\w+)`), s, ""); products != "" {
		s = pbxListAdd(s, products, "children", extAppex+" "+shareExtMarker)
	}
	if main := firstMatch(regexp.MustCompile(`mainGroup = (\w+)`), s, ""); main != "" {
		s = pbxListAdd(s, main, "children", extGroup+" /* ShareExtension */")
	}
	return s
}

// pbxTargetID finds a native target by name.
func pbxTargetID(s, name string) string {
	re := regexp.MustCompile(`(?m)^\t\t(\w+) /\* ` + regexp.QuoteMeta(name) + ` \*/ = \{\n\t\t\tisa = PBXNativeTarget;`)
	return firstMatch(re, s, "")
}

// pbxListAdd adds an item to the end of a list in an object.
func pbxListAdd(s, object, key, item string) string {
	start := regexp.MustCompile(`(?m)^\t\t` + object + `\b[^\n]*= \{\n`).FindStringIndex(s)
	if start == nil {
		return s
	}
	body := s[start[1]:]
	k := strings.Index(body, "\t\t\t"+key+" = (\n")
	stop := strings.Index(body, "\n\t\t};\n")
	if k < 0 || (stop >= 0 && k > stop) {
		return s
	}
	close := strings.Index(body[k:], "\n\t\t\t);")
	if close < 0 {
		return s
	}
	at := start[1] + k + close + 1
	return s[:at] + "\t\t\t\t" + item + ",\n" + s[at:]
}

// pbxDropShareExtension removes every object and reference the extension
// added, in whatever layout Xcode has since saved the file in.
func pbxDropShareExtension(s string) string {
	lines := strings.SplitAfter(s, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		trim := strings.TrimLeft(l, "\t")
		id := ""
		for _, x := range extIDs {
			if strings.HasPrefix(trim, x+" ") || strings.HasPrefix(trim, x+",") || strings.HasPrefix(trim, x+" =") {
				id = x
			}
		}
		if id == "" {
			out = append(out, l)
			continue
		}
		if strings.HasSuffix(strings.TrimSpace(l), "= {") { // an object over many lines: skip to its close
			indent := l[:len(l)-len(trim)]
			for i++; i < len(lines) && lines[i] != indent+"};\n"; i++ {
			}
		}
	}
	return strings.Join(out, "")
}
