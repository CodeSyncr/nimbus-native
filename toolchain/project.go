package toolchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// A project's native apps build with the platform's own build systems, the
// same way the IDEs build them: xcodebuild for native/ios, Gradle for
// native/android. Their build steps call back into `nimbus native lib`.

// IOSDeviceBuild is how to sign a build for a phone.
type IOSDeviceBuild struct {
	Team  string   // Apple team id
	Phone IOSPhone // the device to register and build for
}

// BuildIOSProject builds native/ios for the simulator, or for a phone when
// dev is set (signed automatically with its team), and returns the .app path.
func (e *Env) BuildIOSProject(a *App, t IOSTarget, dev ...IOSDeviceBuild) (string, error) {
	out := filepath.Join(a.BuildDir("ios"), "xcode")
	e.step("xcodebuild → Runner.app (%s)", t.SDK)
	args := []string{"-project", filepath.Join(a.Root, "native/ios/Runner.xcodeproj"), "-scheme", "Runner",
		"-configuration", "Debug", "-sdk", t.SDK, "-derivedDataPath", out}
	if t.SDK == "iphoneos" {
		args = append(args, "-allowProvisioningUpdates", "-allowProvisioningDeviceRegistration")
		if len(dev) > 0 {
			args = append(args, "-destination", "id="+dev[0].Phone.UDID, "DEVELOPMENT_TEAM="+dev[0].Team,
				"CODE_SIGN_STYLE=Automatic")
		}
	} else {
		args = append(args, "-destination", "generic/platform=iOS Simulator", "ONLY_ACTIVE_ARCH=YES", "ARCHS=arm64")
	}
	if a.GoDir != "" {
		args = append(args, "NIMBUS_GO="+filepath.ToSlash(rel(a.Root, a.GoDir)))
	}
	if err := e.run(nil, "xcodebuild", append(args, "build")...); err != nil {
		return "", err
	}
	return filepath.Join(out, "Build/Products", "Debug-"+t.SDK, "Runner.app"), nil
}

// BuildAndroidProject builds native/android's debug APK for the given ABIs
// and returns its path.
func (e *Env) BuildAndroidProject(a *App, abis []string) (string, error) {
	dir := filepath.Join(a.Root, "native/android")
	e.step("gradle → app-debug.apk (%s)", abis[0])
	args := []string{"assembleDebug", "--console=plain"}
	if len(abis) == 1 {
		// What Android Studio passes when it deploys to one device.
		args = append(args, "-Pandroid.injected.build.abi="+abis[0])
	}
	if a.GoDir != "" {
		args = append(args, "-Pnimbus.go="+filepath.ToSlash(rel(a.Root, a.GoDir)))
	}
	if err := e.runIn(dir, nil, filepath.Join(dir, "gradlew"), args...); err != nil {
		return "", err
	}
	return builtAPK(filepath.Join(dir, "app/build"))
}

// builtAPK finds the debug APK. A build for one device's ABI (as Android
// Studio deploys) leaves it under intermediates, named by the metadata
// Android Studio reads.
func builtAPK(build string) (string, error) {
	if apk := filepath.Join(build, "outputs/apk/debug/app-debug.apk"); exists(apk) {
		return apk, nil
	}
	for _, dir := range []string{"outputs/apk/debug", "intermediates/apk/debug"} {
		b, err := os.ReadFile(filepath.Join(build, dir, "output-metadata.json"))
		if err != nil {
			continue
		}
		var meta struct {
			Elements []struct {
				OutputFile string `json:"outputFile"`
			} `json:"elements"`
		}
		if json.Unmarshal(b, &meta) == nil && len(meta.Elements) > 0 {
			if apk := filepath.Join(build, dir, meta.Elements[0].OutputFile); exists(apk) {
				return apk, nil
			}
		}
	}
	return "", fmt.Errorf("gradle built no debug APK under %s", build)
}

// reverseServerPort lets an emulator or device reach the project's server
// (`go run .`, port 3333) at 127.0.0.1, as the app's constants expect.
func (e *Env) reverseServerPort() {
	_ = e.adb("reverse", "tcp:3333", "tcp:3333").Run()
}
