// Package toolchain builds Nimbus Native apps for iOS and Android. Single
// directory apps (the examples) build with the platform tools directly: go
// build (c-archive / c-shared), Xcode's clang and swiftc, the NDK's clang,
// and the Android SDK's aapt2, d8, zipalign and apksigner. Projects made by
// New hold real Xcode and Gradle projects, which call Lib to build the Go
// side. No gomobile either way.
package toolchain

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Env is the set of platform tools found on this machine. Empty fields mean
// the tool was not found; Doctor explains what is missing.
type Env struct {
	Verbose bool
	Out     io.Writer

	Repo string // the nimbus-native module checkout (hosts/, bridge/include/)

	Xcode string // `xcode-select -p`

	AndroidSDK string
	NDK        string // newest ndk/<version>
	BuildTools string // newest build-tools/<version>
	AndroidJar string // newest platforms/android-*/android.jar
	Javac      string
	Keystore   string // debug keystore used to sign dev builds

	// Serial is the Android device to use (adb -s); empty for adb's default.
	Serial string
}

// MinAndroidAPI is the lowest Android version (API 26, Android 8.0) apps run on.
const MinAndroidAPI = 26

// MinIOS is the lowest iOS version apps run on.
const MinIOS = "15.0"

// Detect finds the platform tools. It never fails; Doctor reports gaps.
func Detect(appDir string) *Env {
	e := &Env{Out: os.Stdout}
	e.Repo = moduleDir(appDir)

	if out, err := exec.Command("xcode-select", "-p").Output(); err == nil {
		e.Xcode = strings.TrimSpace(string(out))
	}

	e.AndroidSDK = firstDir(os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT"),
		filepath.Join(home(), "Library/Android/sdk"), filepath.Join(home(), "Android/Sdk"))
	if e.AndroidSDK != "" {
		e.NDK = newest(filepath.Join(e.AndroidSDK, "ndk"))
		e.BuildTools = newest(filepath.Join(e.AndroidSDK, "build-tools"))
		if p := newest(filepath.Join(e.AndroidSDK, "platforms")); p != "" {
			e.AndroidJar = filepath.Join(p, "android.jar")
		}
	}
	if j := os.Getenv("JAVA_HOME"); j != "" && exists(filepath.Join(j, "bin/javac")) {
		e.Javac = filepath.Join(j, "bin/javac")
	} else if p, err := exec.LookPath("javac"); err == nil {
		e.Javac = p
	}
	if k := filepath.Join(home(), ".android/debug.keystore"); exists(k) {
		e.Keystore = k
	}
	return e
}

// Doctor prints what was found and returns false if anything required for
// the given platforms is missing.
func (e *Env) Doctor() bool {
	ok := true
	check := func(name, val, hint string) {
		if val == "" {
			fmt.Fprintf(e.Out, "  ✗ %-14s missing — %s\n", name, hint)
			ok = false
			return
		}
		fmt.Fprintf(e.Out, "  ✓ %-14s %s\n", name, val)
	}
	fmt.Fprintln(e.Out, "Go")
	gov, _ := exec.Command("go", "env", "GOVERSION", "GOHOSTOS", "GOHOSTARCH").Output()
	check("go", strings.Join(strings.Fields(string(gov)), " "), "install Go 1.23+ from go.dev/dl")
	check("nimbus-native", e.Repo, "run inside a module that requires github.com/CodeSyncr/nimbus-native")

	fmt.Fprintln(e.Out, "iOS")
	check("xcode", e.Xcode, "install Xcode, then `sudo xcode-select -s /Applications/Xcode.app`")
	for _, sdk := range []string{"iphoneos", "iphonesimulator"} {
		p, _ := xcrun(sdk, "--show-sdk-path")
		check(sdk, p, "install the iOS platform in Xcode → Settings → Components")
	}

	fmt.Fprintln(e.Out, "Android")
	check("sdk", e.AndroidSDK, "install Android Studio or the command-line tools; set ANDROID_HOME")
	check("ndk", e.NDK, "sdkmanager \"ndk;29.0.14206865\"")
	check("build-tools", e.BuildTools, "sdkmanager \"build-tools;36.1.0\"")
	check("android.jar", e.AndroidJar, "sdkmanager \"platforms;android-36\"")
	check("javac", e.Javac, "install JDK 17+")
	if kc := InstalledKotlinc(); kc != "" {
		check("kotlinc", kc, "")
	} else {
		fmt.Fprintf(e.Out, "  · %-15s Kotlin %s downloads on the first Android build\n", "kotlinc", kotlinVersion)
	}
	check("keystore", e.Keystore, "created by Android Studio; or keytool -genkeypair (see docs)")
	return ok
}

// run executes a tool, echoing the command line in verbose mode and
// including its output in the error when it fails.
func (e *Env) run(env []string, name string, args ...string) error {
	return e.runIn("", env, name, args...)
}

// runIn is run with a working directory.
func (e *Env) runIn(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var buf bytes.Buffer
	if e.Verbose {
		fmt.Fprintf(e.Out, "  $ %s%s %s\n", strings.Join(env, " ")+sep(env), name, strings.Join(args, " "))
		cmd.Stdout, cmd.Stderr = e.Out, e.Out
	} else {
		cmd.Stdout, cmd.Stderr = &buf, &buf
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w\n%s", filepath.Base(name), err, buf.String())
	}
	return nil
}

func (e *Env) step(format string, args ...any) {
	fmt.Fprintf(e.Out, "› "+format+"\n", args...)
}

func xcrun(sdk string, args ...string) (string, error) {
	out, err := exec.Command("xcrun", append([]string{"--sdk", sdk}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// moduleDir finds the nimbus-native checkout the app builds against.
func moduleDir(appDir string) string {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/CodeSyncr/nimbus-native")
	cmd.Dir = appDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func newest(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, en := range entries {
		if en.IsDir() && !strings.HasPrefix(en.Name(), ".") {
			names = append(names, en.Name())
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Slice(names, func(i, j int) bool { return versionLess(names[i], names[j]) })
	return filepath.Join(dir, names[len(names)-1])
}

// versionLess compares "29.0.14206865" / "android-36" style names numerically.
func versionLess(a, b string) bool {
	pa, pb := numbers(a), numbers(b)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return len(pa) < len(pb)
}

func numbers(s string) []int {
	var out []int
	n, in := 0, false
	for _, r := range s + "." {
		if r >= '0' && r <= '9' {
			n, in = n*10+int(r-'0'), true
		} else if in {
			out, n, in = append(out, n), 0, false
		}
	}
	return out
}

func firstDir(paths ...string) string {
	for _, p := range paths {
		if p != "" && exists(p) {
			return p
		}
	}
	return ""
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func home() string { h, _ := os.UserHomeDir(); return h }

func sep(env []string) string {
	if len(env) == 0 {
		return ""
	}
	return " "
}
