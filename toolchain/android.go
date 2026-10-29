package toolchain

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// AndroidABI maps an Android ABI to the Go architecture and NDK compiler.
type AndroidABI struct {
	Name   string // arm64-v8a
	GOARCH string // arm64
	GOARM  string // only for arm
	Clang  string // NDK compiler prefix, without the API level
}

var AndroidABIs = map[string]AndroidABI{
	"arm64-v8a":   {"arm64-v8a", "arm64", "", "aarch64-linux-android"},
	"armeabi-v7a": {"armeabi-v7a", "arm", "7", "armv7a-linux-androideabi"},
	"x86_64":      {"x86_64", "amd64", "", "x86_64-linux-android"},
	"x86":         {"x86", "386", "", "i686-linux-android"},
}

// The shared library every app's Go code is built into; Bridge.java loads it.
const androidLib = "libnimbusapp.so"

// BuildAndroid compiles the Go app into a shared library per ABI, compiles
// the Java host and app code to dex, and assembles a signed, 16 KB-aligned
// APK. It returns the APK path.
func (e *Env) BuildAndroid(a *App, abis []string) (string, error) {
	switch {
	case e.Repo == "":
		return "", fmt.Errorf("nimbus-native module not found from %s", a.Dir)
	case e.NDK == "" || e.BuildTools == "" || e.AndroidJar == "" || e.Javac == "" || e.Keystore == "":
		return "", fmt.Errorf("Android tools missing (run doctor)")
	}
	out := a.BuildDir("android")
	obj := filepath.Join(out, "obj")
	if err := os.RemoveAll(obj); err != nil {
		return "", err
	}
	if err := os.MkdirAll(obj, 0o755); err != nil {
		return "", err
	}
	host := filepath.Join(e.NDK, "toolchains/llvm/prebuilt", ndkHost())

	// 1. Go → libnimbusapp.so for each ABI. Pages are 16 KB-aligned: Google
	// Play requires it for apps targeting Android 15+.
	libs := map[string]string{}
	for _, name := range abis {
		abi, ok := AndroidABIs[name]
		if !ok {
			return "", fmt.Errorf("unknown ABI %q", name)
		}
		e.step("go → %s (android/%s)", androidLib, abi.Name)
		lib := filepath.Join(obj, "lib", abi.Name, androidLib)
		env := []string{"GOOS=android", "GOARCH=" + abi.GOARCH, "CGO_ENABLED=1",
			fmt.Sprintf("CC=%s/bin/%s%d-clang", host, abi.Clang, MinAndroidAPI)}
		if abi.GOARM != "" {
			env = append(env, "GOARM="+abi.GOARM)
		}
		if err := e.goBuild(a.Dir, env, "c-shared", lib, "-extldflags=-Wl,-z,max-page-size=16384"); err != nil {
			return "", err
		}
		libs["lib/"+abi.Name+"/"+androidLib] = lib
	}

	// 2. Java host + app code → classes.dex.
	e.step("javac + d8 → classes.dex")
	classes := filepath.Join(obj, "classes")
	sources := globs(filepath.Join(e.Repo, "hosts/android/src/space/nimbusgo/nativekit/*.java"), filepath.Join(a.Dir, "android/*.java"))
	if err := e.run(nil, e.Javac, append([]string{"--release", "11", "-nowarn", "-classpath", e.AndroidJar, "-d", classes}, sources...)...); err != nil {
		return "", err
	}
	var classFiles []string
	_ = filepath.WalkDir(classes, func(p string, d os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".class") {
			classFiles = append(classFiles, p)
		}
		return nil
	})
	dex := filepath.Join(obj, "dex")
	if err := os.MkdirAll(dex, 0o755); err != nil {
		return "", err
	}
	d8 := append([]string{"--release", "--min-api", fmt.Sprint(MinAndroidAPI), "--lib", e.AndroidJar, "--output", dex}, classFiles...)
	if err := e.run(nil, filepath.Join(e.BuildTools, "d8"), d8...); err != nil {
		return "", err
	}

	// 3. Manifest → base APK (no resources yet).
	e.step("aapt2 → apk")
	manifest := filepath.Join(obj, "AndroidManifest.xml")
	if err := os.WriteFile(manifest, []byte(a.androidManifest()), 0o644); err != nil {
		return "", err
	}
	base := filepath.Join(obj, "base.apk")
	if err := e.run(nil, filepath.Join(e.BuildTools, "aapt2"), "link", "-o", base, "-I", e.AndroidJar,
		"--manifest", manifest, "--min-sdk-version", fmt.Sprint(MinAndroidAPI), "--target-sdk-version", "35",
		"--version-code", "1", "--version-name", "0.1.0"); err != nil {
		return "", err
	}

	// 4. Add dex (compressed) and native libraries (stored, so they can be
	// mapped straight from the APK), then page-align and sign.
	unaligned := filepath.Join(obj, "unaligned.apk")
	add := map[string]string{"classes.dex": filepath.Join(dex, "classes.dex")}
	if err := writeAPK(base, unaligned, add, libs); err != nil {
		return "", err
	}
	aligned := filepath.Join(obj, "aligned.apk")
	if err := e.run(nil, filepath.Join(e.BuildTools, "zipalign"), "-f", "-P", "16", "4", unaligned, aligned); err != nil {
		return "", err
	}
	e.step("apksigner (debug key)")
	apk := filepath.Join(out, a.Name+".apk")
	if err := e.run(nil, filepath.Join(e.BuildTools, "apksigner"), "sign", "--ks", e.Keystore,
		"--ks-pass", "pass:android", "--key-pass", "pass:android", "--ks-key-alias", "androiddebugkey",
		"--out", apk, aligned); err != nil {
		return "", err
	}
	return apk, nil
}

// RunAndroid installs the APK on a connected device or emulator (booting
// the first AVD if none is connected), launches it and streams its logs.
func (e *Env) RunAndroid(a *App, apk string) error {
	adb := filepath.Join(e.AndroidSDK, "platform-tools/adb")
	if err := e.ensureDevice(adb); err != nil {
		return err
	}
	e.step("install %s", filepath.Base(apk))
	if err := e.run(nil, adb, "install", "-r", "-t", "--no-incremental", apk); err != nil {
		return err
	}
	_ = exec.Command(adb, "logcat", "-c").Run()
	e.step("launch %s", a.ID)
	if err := e.run(nil, adb, "shell", "am", "start", "-S", "-n", a.ID+"/.MainActivity"); err != nil {
		return err
	}
	cmd := exec.Command(adb, "logcat", "-v", "brief", "-s", "NimbusNative:*", "NimbusNativeApp:*", "AndroidRuntime:E", "DEBUG:*")
	cmd.Stdout, cmd.Stderr = e.Out, e.Out
	return cmd.Run()
}

func (e *Env) ensureDevice(adb string) error {
	out, err := exec.Command(adb, "devices").Output()
	if err != nil {
		return fmt.Errorf("adb: %w", err)
	}
	if strings.Contains(string(out), "\tdevice") {
		return nil
	}
	avds, _ := exec.Command(filepath.Join(e.AndroidSDK, "emulator/emulator"), "-list-avds").Output()
	sc := bufio.NewScanner(strings.NewReader(string(avds)))
	avd := ""
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" && !strings.Contains(l, " ") {
			avd = l
			break
		}
	}
	if avd == "" {
		return fmt.Errorf("no device connected and no emulator (AVD) configured")
	}
	e.step("boot emulator %s", avd)
	emu := exec.Command(filepath.Join(e.AndroidSDK, "emulator/emulator"), "-avd", avd, "-gpu", "host", "-no-snapshot-load", "-no-audio", "-no-boot-anim")
	if err := emu.Start(); err != nil {
		return fmt.Errorf("start emulator: %w", err)
	}
	_ = exec.Command(adb, "wait-for-device").Run()
	for i := 0; i < 120; i++ {
		b, _ := exec.Command(adb, "shell", "getprop", "sys.boot_completed").Output()
		if strings.TrimSpace(string(b)) == "1" {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("emulator did not finish booting")
}

// writeAPK copies base and adds compressed and stored entries.
func writeAPK(base, out string, deflated, stored map[string]string) error {
	r, err := zip.OpenReader(base)
	if err != nil {
		return err
	}
	defer r.Close()
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for _, entry := range r.File {
		if err := w.Copy(entry); err != nil {
			return err
		}
	}
	add := func(name, src string, method uint16) error {
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		dst, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, in)
		return err
	}
	for name, src := range deflated {
		if err := add(name, src, zip.Deflate); err != nil {
			return err
		}
	}
	for name, src := range stored {
		if err := add(name, src, zip.Store); err != nil {
			return err
		}
	}
	return w.Close()
}

func (a *App) androidManifest() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="` + a.ID + `">
    <application android:label="` + a.Name + `" android:extractNativeLibs="false" android:hasCode="true">
        <activity android:name=".MainActivity" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`
}

// ndkHost is the NDK's prebuilt directory for this OS. The NDK ships only
// x86_64 host binaries; on Apple Silicon they run under Rosetta.
func ndkHost() string {
	return runtime.GOOS + "-x86_64"
}
