package toolchain

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Release builds: what goes to the App Store and Google Play.

// Release says how to build for a store.
type Release struct {
	Version string // shown to users, e.g. "1.2.0"; empty keeps the project's
	Build   string // a number that grows with every upload, e.g. "42"
	Team    string // iOS: the Apple team that signs
	// Export is how the iOS app is signed: "app-store" (the default, for App
	// Store Connect and TestFlight), "ad-hoc" (registered devices) or
	// "development".
	Export string
}

var exportMethods = map[string]string{
	"":            "app-store-connect",
	"app-store":   "app-store-connect",
	"ad-hoc":      "release-testing",
	"development": "debugging",
}

// ReleaseIOS archives the project's Xcode app and exports it as an .ipa,
// signed automatically with the team. It returns the .ipa path.
func (e *Env) ReleaseIOS(a *App, r Release) (string, error) {
	method, ok := exportMethods[r.Export]
	if !ok {
		return "", fmt.Errorf("-export %q: use app-store, ad-hoc or development", r.Export)
	}
	out := filepath.Join(a.BuildDir("ios"), "release")
	if err := os.RemoveAll(out); err != nil {
		return "", err
	}
	archive := filepath.Join(out, a.Name+".xcarchive")
	args := []string{"-project", filepath.Join(a.Root, "native/ios/Runner.xcodeproj"), "-scheme", "Runner",
		"-configuration", "Release", "-destination", "generic/platform=iOS", "-archivePath", archive,
		"-derivedDataPath", filepath.Join(a.BuildDir("ios"), "xcode-release"),
		"-allowProvisioningUpdates", "DEVELOPMENT_TEAM=" + r.Team, "CODE_SIGN_STYLE=Automatic"}
	if r.Version != "" {
		args = append(args, "MARKETING_VERSION="+r.Version)
	}
	if r.Build != "" {
		args = append(args, "CURRENT_PROJECT_VERSION="+r.Build)
	}
	e.step("xcodebuild archive → %s.xcarchive (team %s)", a.Name, r.Team)
	if err := e.run(nil, "xcodebuild", append(args, "archive")...); err != nil {
		return "", err
	}

	plist := filepath.Join(out, "ExportOptions.plist")
	if err := os.WriteFile(plist, []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>method</key><string>%s</string>
	<key>teamID</key><string>%s</string>
	<key>signingStyle</key><string>automatic</string>
	<key>destination</key><string>export</string>
	<key>stripSwiftSymbols</key><true/>
</dict>
</plist>
`, method, r.Team)), 0o644); err != nil {
		return "", err
	}
	e.step("xcodebuild -exportArchive → .ipa (%s)", method)
	if err := e.run(nil, "xcodebuild", "-exportArchive", "-archivePath", archive, "-exportPath", out,
		"-exportOptionsPlist", plist, "-allowProvisioningUpdates"); err != nil {
		if method == "app-store-connect" && (strings.Contains(err.Error(), "No Accounts") || strings.Contains(err.Error(), "No profiles")) {
			return "", fmt.Errorf("%w\n  Xcode could not make an App Store profile for team %s. The Apple ID signed in to\n"+
				"  Xcode (Settings → Accounts) needs the Admin or App Manager role on that team; until then\n"+
				"  `-export development` (or ad-hoc) builds an .ipa, and the archive is in %s", err, r.Team, archive)
		}
		return "", err
	}
	ipas, _ := filepath.Glob(filepath.Join(out, "*.ipa"))
	if len(ipas) == 0 {
		return "", fmt.Errorf("xcodebuild exported no .ipa into %s", out)
	}
	ipa := filepath.Join(out, a.Name+".ipa")
	if ipas[0] != ipa {
		if err := os.Rename(ipas[0], ipa); err != nil {
			return "", err
		}
	}
	return ipa, nil
}

// ReleaseAndroid builds the project's Play Store bundle (.aab) and a
// release APK for every ABI. It returns both paths, and whether they are
// signed with a release key (false: the debug key, which Play refuses).
func (e *Env) ReleaseAndroid(a *App, r Release) (aab, apk string, signed bool, err error) {
	dir := filepath.Join(a.Root, "native/android")
	args := []string{"bundleRelease", "assembleRelease", "--console=plain"}
	if r.Version != "" {
		args = append(args, "-Pnimbus.versionName="+r.Version)
	}
	if r.Build != "" {
		args = append(args, "-Pnimbus.versionCode="+r.Build)
	}
	e.step("gradle → .aab and .apk (release)")
	if err := e.runIn(dir, nil, filepath.Join(dir, "gradlew"), args...); err != nil {
		return "", "", false, err
	}
	out := filepath.Join(a.BuildDir("android"), "release")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", "", false, err
	}
	aab, apk = filepath.Join(out, a.Name+".aab"), filepath.Join(out, a.Name+".apk")
	for src, dst := range map[string]string{
		filepath.Join(dir, "app/build/outputs/bundle/release/app-release.aab"): aab,
		filepath.Join(dir, "app/build/outputs/apk/release/app-release.apk"):    apk,
	} {
		b, err := os.ReadFile(src)
		if err != nil {
			return "", "", false, err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return "", "", false, err
		}
	}
	return aab, apk, exists(filepath.Join(dir, "keystore.properties")), nil
}

// MakeKeystore creates the project's Android release key (an "upload key"
// for Google Play) with keytool, and keystore.properties pointing at it.
// Both stay out of git; back them up.
func (e *Env) MakeKeystore(a *App) (string, error) {
	dir := filepath.Join(a.Root, "native/android")
	store := filepath.Join(dir, "release.keystore")
	props := filepath.Join(dir, "keystore.properties")
	if exists(store) || exists(props) {
		return "", fmt.Errorf("%s already has a release key (release.keystore, keystore.properties): keep using it", dir)
	}
	keytool := "keytool"
	if j := os.Getenv("JAVA_HOME"); j != "" && exists(filepath.Join(j, "bin/keytool")) {
		keytool = filepath.Join(j, "bin/keytool")
	} else if e.Javac != "" {
		keytool = filepath.Join(filepath.Dir(e.Javac), "keytool")
	}
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	pass := base64.RawURLEncoding.EncodeToString(b)
	name := strings.ReplaceAll(a.Name, ",", " ")
	if err := e.run(nil, keytool, "-genkeypair", "-keystore", store, "-alias", "upload", "-keyalg", "RSA",
		"-keysize", "2048", "-validity", "10000", "-storepass", pass, "-keypass", pass,
		"-dname", "CN="+name+", O="+name); err != nil {
		return "", fmt.Errorf("%w (keytool comes with the JDK)", err)
	}
	content := fmt.Sprintf("# The release (upload) key for Google Play, made by nimbus native keystore.\n"+
		"# Kept out of git: back up this file and release.keystore. Losing them means\n"+
		"# asking Google Play support for a new upload key.\n"+
		"storeFile=release.keystore\nstorePassword=%s\nkeyAlias=upload\nkeyPassword=%s\n", pass, pass)
	if err := os.WriteFile(props, []byte(content), 0o600); err != nil {
		return "", err
	}
	return store, nil
}
