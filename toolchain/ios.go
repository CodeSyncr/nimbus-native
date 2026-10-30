package toolchain

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IOSTarget is where an iOS build runs.
type IOSTarget struct {
	SDK    string // iphonesimulator | iphoneos
	Triple string // clang/swiftc target
}

var (
	IOSSimulator = IOSTarget{"iphonesimulator", "arm64-apple-ios" + MinIOS + "-simulator"}
	IOSDevice    = IOSTarget{"iphoneos", "arm64-apple-ios" + MinIOS}
)

// BuildIOS compiles the Go app into a static library, links it with the
// NimbusNativeKit host and the app's Swift code, and produces a signed .app.
// It returns the .app path.
func (e *Env) BuildIOS(a *App, t IOSTarget) (string, error) {
	if e.Repo == "" {
		return "", fmt.Errorf("nimbus-native module not found from %s", a.Dir)
	}
	sdkPath, err := xcrun(t.SDK, "--show-sdk-path")
	if err != nil {
		return "", fmt.Errorf("iOS SDK %s not found (run doctor)", t.SDK)
	}

	out := filepath.Join(a.BuildDir("ios"), t.SDK)
	obj := filepath.Join(out, "obj")
	app := filepath.Join(out, a.Name+".app")
	if err := os.MkdirAll(obj, 0o755); err != nil {
		return "", err
	}
	if err := os.RemoveAll(app); err != nil {
		return "", err
	}
	if err := os.MkdirAll(app, 0o755); err != nil {
		return "", err
	}

	// 0. Wire schemas → Go, Swift.
	generated, err := e.Gen(a)
	if err != nil {
		return "", err
	}

	// 1. Go → static library. cgo compiles with Xcode's clang for the target.
	e.step("go → libapp.a (ios/arm64, %s)", t.SDK)
	if err := e.goIOS(a, t.SDK, "arm64", MinIOS, filepath.Join(obj, "libapp.a")); err != nil {
		return "", err
	}

	// 2. Swift host + app code, linked against the Go library.
	e.step("swiftc → %s", a.Name)
	sources := globs(filepath.Join(e.Repo, "hosts/ios/Sources/NimbusNativeKit/*.swift"), filepath.Join(a.Dir, "ios/*.swift"))
	sources = append(sources, generated.SwiftFiles...)
	if len(sources) == 0 {
		return "", fmt.Errorf("no Swift sources in %s/ios", a.Dir)
	}
	args := []string{"--sdk", t.SDK, "swiftc",
		"-target", t.Triple, "-sdk", sdkPath, "-module-name", a.Name, "-O",
		"-import-objc-header", filepath.Join(e.Repo, "bridge/include/nn.h"),
		"-L", obj, "-lapp", "-lresolv",
		"-o", filepath.Join(app, a.Name)}
	if err := e.run(nil, "xcrun", append(args, sources...)...); err != nil {
		return "", err
	}

	// 3. Bundle + sign. Simulator builds take an ad-hoc signature; device
	// builds will need a real identity and profile (Phase 2).
	if err := os.WriteFile(filepath.Join(app, "Info.plist"), []byte(a.infoPlist(t)), 0o644); err != nil {
		return "", err
	}
	e.step("codesign (ad hoc)")
	if err := e.run(nil, "codesign", "--force", "--sign", "-", "--timestamp=none", app); err != nil {
		return "", err
	}
	return app, nil
}

// goIOS builds the app's Go code into a static library for one SDK and
// architecture, compiling cgo with Xcode's clang.
func (e *Env) goIOS(a *App, sdk, arch, minOS, out string) error {
	sdkPath, err := xcrun(sdk, "--show-sdk-path")
	if err != nil {
		return fmt.Errorf("iOS SDK %s not found (run doctor)", sdk)
	}
	clang, err := xcrun(sdk, "--find", "clang")
	if err != nil {
		return fmt.Errorf("clang for %s not found", sdk)
	}
	goarch, triple := "arm64", "arm64-apple-ios"+minOS
	if arch == "x86_64" {
		goarch, triple = "amd64", "x86_64-apple-ios"+minOS
	}
	if sdk == "iphonesimulator" {
		triple += "-simulator"
	}
	cc := fmt.Sprintf("%s -isysroot %s -target %s", clang, sdkPath, triple)
	env := []string{"GOOS=ios", "GOARCH=" + goarch, "CGO_ENABLED=1", "CC=" + cc}
	return e.goBuild(a.goDir(), env, "c-archive", out)
}

// RunIOS installs the app on a simulator (booting one if needed), launches
// it and streams its console until it exits or the command is interrupted.
func (e *Env) RunIOS(a *App, appPath string) error {
	udid, err := bootedSimulator()
	if err != nil {
		return err
	}
	e.step("install on simulator %s", udid)
	if err := e.run(nil, "xcrun", "simctl", "install", udid, appPath); err != nil {
		return err
	}
	e.step("launch %s", a.ID)
	cmd := exec.Command("xcrun", "simctl", "launch", "--console-pty", "--terminate-running-process", udid, a.ID)
	cmd.Stdout, cmd.Stderr = e.Out, e.Out
	return cmd.Run()
}

// bootedSimulator returns a booted iPhone simulator (on the newest iOS when
// several are), booting the newest available one when none is running.
// Devices are told apart by type, not name, which can be anything.
func bootedSimulator() (string, error) {
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "available", "-j").Output()
	if err != nil {
		return "", fmt.Errorf("simctl: %w", err)
	}
	booted, newest := pickSimulator(out)
	if booted != "" {
		return booted, nil
	}
	if newest == "" {
		return "", fmt.Errorf("no iPhone simulator installed (Xcode → Settings → Components)")
	}
	if err := exec.Command("xcrun", "simctl", "boot", newest).Run(); err != nil {
		return "", fmt.Errorf("boot simulator %s: %w", newest, err)
	}
	_ = exec.Command("open", "-a", "Simulator").Run()
	return newest, nil
}

// pickSimulator reads `simctl list devices available -j`: the booted iPhone
// on the newest iOS, and the newest iPhone of all.
func pickSimulator(listJSON []byte) (booted, newest string) {
	var doc struct {
		Devices map[string][]struct {
			UDID, State, DeviceTypeIdentifier string
		} `json:"devices"`
	}
	if json.Unmarshal(listJSON, &doc) != nil {
		return "", ""
	}
	// Runtimes are keys like com.apple.CoreSimulator.SimRuntime.iOS-26-3.
	version := func(rt string) [2]int {
		var v [2]int
		if i := strings.LastIndex(rt, "iOS-"); i >= 0 {
			fmt.Sscanf(strings.ReplaceAll(rt[i+4:], "-", " "), "%d %d", &v[0], &v[1])
		}
		return v
	}
	var bootedV, newestV [2]int
	later := func(a, b [2]int) bool { return a[0] > b[0] || a[0] == b[0] && a[1] > b[1] }
	for rt, devs := range doc.Devices {
		if !strings.Contains(rt, ".iOS-") {
			continue
		}
		v := version(rt)
		for _, d := range devs {
			if !strings.Contains(d.DeviceTypeIdentifier, "iPhone") {
				continue
			}
			if d.State == "Booted" && (booted == "" || later(v, bootedV)) {
				booted, bootedV = d.UDID, v
			}
			if newest == "" || later(v, newestV) {
				newest, newestV = d.UDID, v
			}
		}
	}
	return booted, newest
}

func between(s, open, close string) string {
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i+1:], close)
	if j < 0 {
		return ""
	}
	return s[i+1 : i+1+j]
}

func (a *App) infoPlist(t IOSTarget) string {
	platform := "iPhoneOS"
	if t.SDK == "iphonesimulator" {
		platform = "iPhoneSimulator"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key><string>` + a.ID + `</string>
	<key>CFBundleName</key><string>` + a.Name + `</string>
	<key>CFBundleDisplayName</key><string>` + a.Name + `</string>
	<key>CFBundleExecutable</key><string>` + a.Name + `</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>CFBundleShortVersionString</key><string>0.1.0</string>
	<key>CFBundleSupportedPlatforms</key><array><string>` + platform + `</string></array>
	<key>MinimumOSVersion</key><string>` + MinIOS + `</string>
	<key>UIDeviceFamily</key><array><integer>1</integer><integer>2</integer></array>
	<key>UIRequiredDeviceCapabilities</key><array><string>arm64</string></array>
	<key>UILaunchScreen</key><dict/>
</dict>
</plist>
`
}
