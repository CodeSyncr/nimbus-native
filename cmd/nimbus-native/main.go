// Command nimbus-native is the Nimbus Native toolchain behind `nimbus native`:
// the nimbus CLI runs the version of this command that an app depends on,
// so users never call it directly.
//
//	nimbus native new <dir>
//	nimbus native doctor
//	nimbus native gen [app-dir]
//	nimbus native build ios|android [flags] [app-dir]
//	nimbus native run   ios|android [flags] [app-dir]
//	nimbus native dev   ios|android [-port N] [app-dir]
//	nimbus native lib   ios|android [flags] [project-dir]   (run by Xcode and Gradle)
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/CodeSyncr/nimbus-native/toolchain"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	switch cmd {
	case "doctor":
		if !toolchain.Detect(".").Doctor() {
			os.Exit(1)
		}
	case "gen":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		app, err := toolchain.LoadApp(dir, "", "")
		if err == nil {
			var g *toolchain.Generated
			if g, err = toolchain.Detect(app.Dir).Gen(app); err == nil {
				fmt.Printf("schema %#016x\n", g.Combined)
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "dev":
		if len(os.Args) < 3 {
			usage()
		}
		if err := dev(os.Args[2], os.Args[3:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "new":
		if err := newProject(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "lib":
		if len(os.Args) < 3 {
			usage()
		}
		if err := lib(os.Args[2], os.Args[3:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "build", "run":
		if len(os.Args) < 3 {
			usage()
		}
		if err := buildOrRun(cmd, os.Args[2], os.Args[3:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		usage()
	}
}

func buildOrRun(cmd, platform string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	name := fs.String("name", "", "app name (default: directory name)")
	id := fs.String("id", "", "bundle / application id (default: space.nimbusgo.<dir>)")
	device := fs.Bool("device", false, "iOS: build for a device instead of the simulator")
	abis := fs.String("abi", "arm64-v8a", "Android: comma-separated ABIs (arm64-v8a,armeabi-v7a,x86_64,x86)")
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}

	if toolchain.IsProject(dir) {
		return buildOrRunProject(cmd, platform, dir, *device, strings.Split(*abis, ","), *verbose)
	}
	app, err := toolchain.LoadApp(dir, *name, *id)
	if err != nil {
		return err
	}
	env := toolchain.Detect(app.Dir)
	env.Verbose = *verbose

	switch platform {
	case "ios":
		target := toolchain.IOSSimulator
		if *device {
			target = toolchain.IOSDevice
		}
		out, err := env.BuildIOS(app, target)
		if err != nil {
			return err
		}
		fmt.Println("built", out)
		if cmd == "run" {
			if *device {
				return fmt.Errorf("running on a device is not supported yet; install %s with Xcode", out)
			}
			return env.RunIOS(app, out)
		}
	case "android":
		out, err := env.BuildAndroid(app, strings.Split(*abis, ","))
		if err != nil {
			return err
		}
		fmt.Println("built", out)
		if cmd == "run" {
			return env.RunAndroid(app, out)
		}
	default:
		return fmt.Errorf("unknown platform %q (ios or android)", platform)
	}
	return nil
}

// buildOrRunProject builds a project's native/ios or native/android with
// xcodebuild or Gradle, as the IDEs do.
func buildOrRunProject(cmd, platform, dir string, device bool, abis []string, verbose bool) error {
	app, err := toolchain.LoadProject(dir, "app/mobile", "")
	if err != nil {
		return err
	}
	env := toolchain.Detect(app.Root)
	env.Verbose = verbose
	switch platform {
	case "ios":
		if app.ID, err = toolchain.IOSProjectID(app.Root); err != nil {
			return err
		}
		target := toolchain.IOSSimulator
		if device {
			target = toolchain.IOSDevice
		}
		out, err := env.BuildIOSProject(app, target)
		if err != nil {
			return err
		}
		fmt.Println("built", out)
		if cmd == "run" {
			if device {
				return fmt.Errorf("running on a device is not supported yet; run it from Xcode (native/ios/Runner.xcodeproj)")
			}
			return env.RunIOS(app, out)
		}
	case "android":
		if app.ID, err = toolchain.AndroidProjectID(app.Root); err != nil {
			return err
		}
		out, err := env.BuildAndroidProject(app, abis)
		if err != nil {
			return err
		}
		fmt.Println("built", out)
		if cmd == "run" {
			return env.RunAndroid(app, out)
		}
	default:
		return fmt.Errorf("unknown platform %q (ios or android)", platform)
	}
	return nil
}

// newProject creates a project.
func newProject(args []string) error {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	name := fs.String("name", "", "app name (default: from the directory)")
	module := fs.String("module", "", "Go module path (default: the directory name)")
	id := fs.String("id", "", "bundle / application id (default: space.nimbusgo.<dir>)")
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: nimbus native new [-name N] [-module M] [-id ID] <dir>")
	}
	p, err := toolchain.NewProjectFor(fs.Arg(0), *name, *module, *id)
	if err != nil {
		return err
	}
	env := toolchain.Detect(".")
	env.Verbose = *verbose
	if err := env.New(p); err != nil {
		return err
	}
	fmt.Printf(`
created %s (%s, %s)

  cd %s
  go run .                          start the server (http://127.0.0.1:3333)
  nimbus native run ios             build and run on the iOS simulator
  nimbus native run android         build and run on an Android emulator

  open native/ios/Runner.xcodeproj  or open native/android in Android Studio
`, p.Dir, p.Name, p.ID, fs.Arg(0))
	return nil
}

// lib builds a project's Go library and generated host code; the Xcode and
// Gradle projects run it as a build step. Defaults come from Xcode's
// build settings when it is the caller.
func lib(platform string, args []string) error {
	fs := flag.NewFlagSet("lib", flag.ExitOnError)
	entry := fs.String("entry", "app/mobile", "the phone app's package main, relative to the project")
	id := fs.String("id", "", "Android: the app's namespace (the generated Kotlin's package)")
	sdk := fs.String("sdk", envOr("PLATFORM_NAME", "iphonesimulator"), "iOS: iphonesimulator or iphoneos")
	archs := fs.String("arch", envOr("ARCHS", "arm64"), "iOS: architectures, space or comma separated")
	minIOS := fs.String("min-ios", envOr("IPHONEOS_DEPLOYMENT_TARGET", toolchain.MinIOS), "iOS: deployment target")
	out := fs.String("out", "", "iOS: directory for libapp.a")
	swift := fs.String("swift", "", "iOS: the generated Swift file to write")
	abis := fs.String("abi", "arm64-v8a", "Android: comma-separated ABIs")
	jni := fs.String("jni", "", "Android: jniLibs directory to write")
	kotlin := fs.String("kotlin", "", "Android: source directory for the generated Kotlin")
	kit := fs.String("kit", "", "the project's copy of the host kit, kept in sync with go.mod")
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	app, err := toolchain.LoadProject(dir, *entry, *id)
	if err != nil {
		return err
	}
	env := toolchain.Detect(app.Root)
	env.Verbose = *verbose
	switch platform {
	case "ios":
		if *out == "" || *swift == "" {
			return fmt.Errorf("lib ios needs -out and -swift")
		}
		err = env.LibIOS(app, toolchain.IOSLib{SDK: *sdk, Archs: strings.FieldsFunc(*archs, func(r rune) bool { return r == ' ' || r == ',' }),
			MinOS: *minIOS, Out: *out, Swift: *swift, Kit: *kit})
	case "android":
		if *jni == "" || *kotlin == "" || app.ID == "" {
			return fmt.Errorf("lib android needs -id, -jni and -kotlin")
		}
		err = env.LibAndroid(app, toolchain.AndroidLib{ABIs: strings.Split(*abis, ","), JNI: *jni, Kotlin: *kotlin, Kit: *kit})
	default:
		return fmt.Errorf("unknown platform %q (ios or android)", platform)
	}
	return err
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// dev runs the app on this machine against a dev shell, reloading on save.
func dev(platform string, args []string) error {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	port := fs.Int("port", 7357, "port for the dev relay")
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	app, err := toolchain.LoadApp(dir, "", "")
	if err != nil {
		return err
	}
	env := toolchain.Detect(app.Dir)
	env.Verbose = *verbose
	return env.Dev(app, platform, *port)
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  nimbus native new <dir>                              a project with native/ios and native/android
  nimbus native doctor
  nimbus native gen [app-dir]
  nimbus native build ios|android [-device] [-abi list] [-name N] [-id ID] [-v] [app-dir]
  nimbus native run   ios|android [...same flags] [app-dir]
  nimbus native dev   ios|android [-port N] [app-dir]   run on this machine, reload on save
  nimbus native lib   ios|android [flags] [project-dir]   the Go side of a project build (Xcode and Gradle run it)`)
	os.Exit(2)
}
