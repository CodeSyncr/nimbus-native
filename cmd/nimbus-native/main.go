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
	"os/exec"
	"path/filepath"
	"sort"
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
		var app *toolchain.App
		err := inModule(dir)
		if err == nil && toolchain.IsProject(dir) {
			id, _ := toolchain.AndroidProjectID(dir) // the generated Kotlin's package
			app, err = toolchain.LoadProject(dir, "app/mobile", id)
		} else if err == nil {
			app, err = toolchain.LoadApp(dir, "", "")
		}
		if err == nil {
			var g *toolchain.Generated
			if g, err = toolchain.Detect(app.Dir).Gen(app); err == nil {
				fmt.Printf("schema %#016x\n", g.Combined)
			}
		}
		if err != nil {
			toolchain.Errorf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	case "dev":
		if len(os.Args) < 3 {
			usage()
		}
		if err := dev(os.Args[2], os.Args[3:]); err != nil {
			toolchain.Errorf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	case "new":
		if err := newProject(os.Args[2:]); err != nil {
			toolchain.Errorf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	case "upgrade", "sync":
		var err error
		if cmd == "upgrade" {
			err = upgrade(os.Args[2:])
		} else {
			err = syncProject(os.Args[2:])
		}
		if err != nil {
			toolchain.Errorf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	case "keystore":
		if err := keystore(os.Args[2:]); err != nil {
			toolchain.Errorf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	case "font":
		if err := font(os.Args[2:]); err != nil {
			toolchain.Errorf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	case "icon":
		if err := icon(os.Args[2:]); err != nil {
			toolchain.Errorf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	case "lib":
		if len(os.Args) < 3 {
			usage()
		}
		if err := lib(os.Args[2], os.Args[3:]); err != nil {
			toolchain.Errorf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	case "build", "run":
		if len(os.Args) < 3 {
			usage()
		}
		if err := buildOrRun(cmd, os.Args[2], os.Args[3:]); err != nil {
			toolchain.Errorf(os.Stderr, "%v", err)
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
	device := fs.Bool("device", false, "build for (and run on) a phone; run picks one by default when connected")
	sim := fs.Bool("sim", false, "run on the simulator / emulator even when a phone is connected")
	udid := fs.String("udid", "", "which phone (or emulator), by name, id or serial")
	team := fs.String("team", "", "iOS: the Apple team that signs device builds (or NIMBUS_IOS_TEAM)")
	release := fs.Bool("release", false, "build: a store build (.ipa for the App Store, .aab for Google Play)")
	version := fs.String("version", "", "release: the version users see, e.g. 1.2.0")
	buildNo := fs.String("build", "", "release: the build number, higher for every upload, e.g. 42")
	export := fs.String("export", "app-store", "iOS release: app-store, ad-hoc or development")
	abis := fs.String("abi", "arm64-v8a", "Android: comma-separated ABIs (arm64-v8a,armeabi-v7a,x86_64,x86)")
	server := fs.String("server", "", "the address of the app's server, built into the app (or NIMBUS_SERVER_URL)")
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}

	if err := inModule(dir); err != nil {
		return err
	}
	if *server != "" {
		if err := toolchain.SetServer(*server); err != nil {
			return err
		}
	}
	if *release {
		if cmd != "build" || !toolchain.IsProject(dir) {
			return fmt.Errorf("-release goes with `nimbus native build` in a project")
		}
		if toolchain.HasServer(dir) && !toolchain.ServerSet() {
			toolchain.Warnf(os.Stdout, "No server address given: the app will use the one in its code. Pass -server https://your-server")
		}
		return buildRelease(platform, dir, toolchain.Release{Version: *version, Build: *buildNo, Team: *team, Export: *export}, *verbose)
	}
	if toolchain.IsProject(dir) {
		return buildOrRunProject(cmd, platform, dir, *device, *sim, *udid, *team, strings.Split(*abis, ","), *verbose)
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
		toolchain.Successf(os.Stdout, "Built %s", out)
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
		toolchain.Successf(os.Stdout, "Built %s", out)
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
func buildOrRunProject(cmd, platform, dir string, device, sim bool, udid, team string, abis []string, verbose bool) error {
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
		phone, onPhone, err := pickPhone(cmd, device, sim, udid)
		if err != nil {
			return err
		}
		if !onPhone {
			if cmd == "run" {
				if err := useLocalServer(app.Root, toolchain.OnSimulator); err != nil {
					return err
				}
			}
			out, err := env.BuildIOSProject(app, toolchain.IOSSimulator)
			if err != nil {
				return err
			}
			toolchain.Successf(os.Stdout, "Built %s", out)
			if cmd == "run" {
				return env.RunIOS(app, out)
			}
			return nil
		}
		if team, err = pickTeam(app.Root, team); err != nil {
			return err
		}
		via := "USB"
		if phone.Wifi {
			via = "Wi-Fi"
		}
		toolchain.Infof(os.Stdout, "%s over %s, signed by team %s", phone.Name, via, team)
		build := toolchain.IOSDeviceBuild{Team: team, Phone: phone}
		if cmd == "run" {
			if err := useLocalServer(app.Root, toolchain.OnPhone); err != nil {
				return err
			}
			if toolchain.ServerSet() {
				// iOS asks once before an app talks to the local network.
				build.Settings = []string{"INFOPLIST_KEY_NSLocalNetworkUsageDescription=The app talks to its server on your local network."}
			}
		}
		out, err := env.BuildIOSProject(app, toolchain.IOSDevice, build)
		if err != nil {
			return err
		}
		toolchain.Successf(os.Stdout, "Built %s", out)
		if cmd == "run" {
			return env.RunIOSPhone(app, out, phone)
		}
	case "android":
		if app.ID, err = toolchain.AndroidProjectID(app.Root); err != nil {
			return err
		}
		if cmd == "run" {
			if env.Serial, err = pickAndroid(env, device, sim, udid); err != nil {
				return err
			}
			target := toolchain.OnPhone
			if env.Serial == "" || strings.HasPrefix(env.Serial, "emulator-") {
				target = toolchain.OnEmulator
			}
			if err := useLocalServer(app.Root, target); err != nil {
				return err
			}
		}
		out, err := env.BuildAndroidProject(app, abis)
		if err != nil {
			return err
		}
		toolchain.Successf(os.Stdout, "Built %s", out)
		if cmd == "run" {
			return env.RunAndroid(app, out)
		}
	default:
		return fmt.Errorf("unknown platform %q (ios or android)", platform)
	}
	return nil
}

// useLocalServer points a run at the project's server on this Mac, unless
// it has been given another or the project has none.
func useLocalServer(root string, target int) error {
	if toolchain.ServerSet() || !toolchain.HasServer(root) {
		return nil
	}
	url := toolchain.LocalServer(root, target)
	if url == "" {
		toolchain.Warnf(os.Stdout, "This Mac isn't on a network, so the phone can't reach its server. Pass -server https://your-server")
		return nil
	}
	toolchain.Infof(os.Stdout, "The app will look for its server at %s: start it with `nimbus serve`", url)
	return toolchain.SetServer(url)
}

// newProject creates a project.
func newProject(args []string) error {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	name := fs.String("name", "", "app name (default: from the directory)")
	module := fs.String("module", "", "Go module path (default: the directory name)")
	id := fs.String("id", "", "bundle / application id (default: space.nimbusgo.<dir>)")
	template := fs.String("template", "counter", "the app to start from: "+strings.Join(toolchain.Templates, ", "))
	server := fs.String("server", "none", "none: the phone app alone, for any backend; nimbus: add the app to the Nimbus app in <dir> (nimbus native new does this)")
	quiet := fs.Bool("quiet", false, "no closing summary (for nimbus native new)")
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(reorder(args))
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: nimbus native new [-server none|nimbus] [-name N] [-module M] [-id ID] <dir>")
	}
	p, err := toolchain.NewProjectFor(fs.Arg(0), *name, *module, *id)
	if err != nil {
		return err
	}
	p.Template, p.Server = *template, *server
	env := toolchain.Detect(".")
	env.Verbose = *verbose
	if err := env.New(p); err != nil {
		return err
	}
	if *quiet {
		return nil
	}
	toolchain.Successf(os.Stdout, "Created %s (%s, %s): the phone app, with no server", p.Dir, p.Name, p.ID)
	toolchain.Panel(os.Stdout, "Next steps", fmt.Sprintf(`cd %s
nimbus native dev ios             the app on your iPhone or the simulator, reloading on save
nimbus native dev android         the same on Android

open native/ios/Runner.xcodeproj  or open native/android in Android Studio

The app reaches your backend (Node, Go, anything that speaks HTTP and JSON)
with api.New("https://your-server"): see features/ for where calls go.`, fs.Arg(0)))
	return nil
}

// upgrade moves the project to the latest release, or the one named.
func upgrade(args []string) error {
	fs := flag.NewFlagSet("upgrade", flag.ExitOnError)
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(reorder(args))
	version := fs.Arg(0)
	if err := inModule("."); err != nil {
		return err
	}
	if !toolchain.IsProject(".") {
		return fmt.Errorf("upgrade works in a project made by `nimbus native new` (with native/ios and native/android)")
	}
	env := toolchain.Detect(".")
	env.Verbose = *verbose
	from, to, err := env.Upgrade(".", version)
	if err != nil {
		return err
	}
	if from == to {
		toolchain.Infof(os.Stdout, "Already on %s.", to)
		return nil
	}
	toolchain.Successf(os.Stdout, "Upgraded %s → %s. What changed: https://github.com/CodeSyncr/nimbus-native/blob/main/CHANGELOG.md", from, to)
	return nil
}

// syncProject updates the project's kits and generated code from the
// nimbus-native its go.mod requires (upgrade runs it with the new release).
func syncProject(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	if err := inModule(dir); err != nil {
		return err
	}
	id, _ := toolchain.AndroidProjectID(dir)
	app, err := toolchain.LoadProject(dir, "app/mobile", id)
	if err != nil {
		return err
	}
	_, err = toolchain.Detect(app.Dir).Sync(app)
	return err
}

// buildRelease makes the store builds.
func buildRelease(platform, dir string, r toolchain.Release, verbose bool) error {
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
		if r.Team, err = pickTeam(app.Root, r.Team); err != nil {
			return err
		}
		ipa, err := env.ReleaseIOS(app, r)
		if err != nil {
			return err
		}
		toolchain.Successf(os.Stdout, "Built %s", ipa)
		toolchain.Panel(os.Stdout, "Upload it", fmt.Sprintf("With Xcode's Organizer or Transporter, or:\n\nxcrun altool --upload-app -f %q -t ios --apiKey <KEY> --apiIssuer <ISSUER>", ipa))
	case "android":
		if app.ID, err = toolchain.AndroidProjectID(app.Root); err != nil {
			return err
		}
		aab, apk, signed, err := env.ReleaseAndroid(app, r)
		if err != nil {
			return err
		}
		toolchain.Successf(os.Stdout, "Built %s  (Google Play)", aab)
		toolchain.Successf(os.Stdout, "Built %s  (install directly)", apk)
		if !signed {
			toolchain.Warnf(os.Stdout, "Signed with the debug key: Google Play won't accept it. Make a release key once with\n  `nimbus native keystore`, then build again.")
		}
	default:
		return fmt.Errorf("unknown platform %q (ios or android)", platform)
	}
	return nil
}

// keystore makes the project's Android release key.
func keystore(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	if !toolchain.IsProject(dir) {
		return inModule(dir)
	}
	app, err := toolchain.LoadProject(dir, "app/mobile", "")
	if err != nil {
		return err
	}
	store, err := toolchain.Detect(app.Root).MakeKeystore(app)
	if err != nil {
		return err
	}
	toolchain.Successf(os.Stdout, "Made %s and keystore.properties next to it", store)
	toolchain.Infof(os.Stdout, "They stay out of git. Back both up somewhere safe: Google Play knows your app by this key.")
	return nil
}

// font adds a bundled font to the project's apps.
func font(args []string) error {
	fs := flag.NewFlagSet("font", flag.ExitOnError)
	name := fs.String("name", "", "the family name to use in Style.FontFamily (default: the file name)")
	_ = fs.Parse(reorder(args))
	if fs.NArg() < 2 || fs.Arg(0) != "add" {
		return fmt.Errorf("usage: nimbus native font add [-name Family] <font.ttf|font.otf> [project-dir]")
	}
	dir := "."
	if fs.NArg() > 2 {
		dir = fs.Arg(2)
	}
	if !toolchain.IsProject(dir) {
		return inModule(dir)
	}
	family, err := toolchain.AddFont(dir, fs.Arg(1), *name)
	if err != nil {
		return err
	}
	toolchain.Successf(os.Stdout, "Added %q to the iOS and Android apps (and the web)", family)
	toolchain.Infof(os.Stdout, "Use it: ui.Style{FontFamily: %q}", family)
	return nil
}

// icon makes a project's app icons and launch screens from one picture.
func icon(args []string) error {
	fs := flag.NewFlagSet("icon", flag.ExitOnError)
	bg := fs.String("background", "#ffffff", "the color behind the icon (iOS icons can't be transparent)")
	launch := fs.String("launch", "", "the launch screen's color (default: keep the current one)")
	_ = fs.Parse(reorder(args))
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: nimbus native icon [-background #hex] [-launch #hex] <image.png> [project-dir]")
	}
	dir := "."
	if fs.NArg() > 1 {
		dir = fs.Arg(1)
	}
	if !toolchain.IsProject(dir) {
		return inModule(dir) // says what to do, or: not a project
	}
	img, err := toolchain.ReadIcon(fs.Arg(0))
	if err != nil {
		return err
	}
	b := img.Bounds()
	if b.Dx() < 1024 {
		toolchain.Warnf(os.Stdout, "%s is %d px: 1024 px or more looks sharper on the App Store", fs.Arg(0), b.Dx())
	}
	back, err := toolchain.ParseColor(*bg)
	if err != nil {
		return err
	}
	launchColor := toolchain.LaunchColor(dir)
	if *launch != "" {
		if launchColor, err = toolchain.ParseColor(*launch); err != nil {
			return err
		}
	}
	if err := toolchain.WriteIcons(dir, img, back, launchColor); err != nil {
		return err
	}
	toolchain.Successf(os.Stdout, "Wrote the iOS icon (native/ios/Runner/Assets.xcassets) and Android icons (native/android/app/src/main/res)")
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
	goPkg := fs.String("go", "", "build this package (relative to the project) into the library instead of -entry; dev mode's proxy")
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
	if *goPkg != "" {
		app.GoDir = filepath.Join(app.Root, filepath.FromSlash(*goPkg))
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
// Like run, the shell goes on a connected phone when there is one.
func dev(platform string, args []string) error {
	if platform != "ios" && platform != "android" {
		return fmt.Errorf("usage: nimbus native dev ios|android [flags] [dir]")
	}
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	port := fs.Int("port", 7357, "port for the dev relay")
	device := fs.Bool("device", false, "use a phone (the default when one is connected)")
	sim := fs.Bool("sim", false, "use the simulator / emulator even when a phone is connected")
	udid := fs.String("udid", "", "which phone (or emulator), by name, id or serial")
	team := fs.String("team", "", "iOS: the Apple team that signs the phone build")
	noServer := fs.Bool("no-server", false, "don't run the project's Nimbus server (go run .) alongside")
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(reorder(args))
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if err := inModule(dir); err != nil {
		return err
	}
	var app *toolchain.App
	var err error
	project := toolchain.IsProject(dir)
	if project {
		var id string
		switch platform {
		case "ios":
			id, err = toolchain.IOSProjectID(dir)
		case "android":
			id, err = toolchain.AndroidProjectID(dir)
		}
		if err != nil {
			return err
		}
		app, err = toolchain.LoadProject(dir, "app/mobile", id)
	} else {
		app, err = toolchain.LoadApp(dir, "", "")
	}
	if err != nil {
		return err
	}
	env := toolchain.Detect(app.Dir)
	env.Verbose = *verbose

	var target toolchain.DevTarget
	switch platform {
	case "ios":
		phone, onPhone, err := pickPhone("run", *device, *sim || !project, *udid)
		if err != nil {
			return err
		}
		if onPhone {
			t, err := pickTeam(app.Root, *team)
			if err != nil {
				return err
			}
			target = toolchain.DevTarget{Phone: &phone, Team: t}
		}
	case "android":
		if env.Serial, err = pickAndroid(env, *device, *sim, *udid); err != nil {
			return err
		}
	}
	target.NoServer = *noServer
	return env.Dev(app, platform, *port, target)
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  nimbus native new <dir>                              a project with native/ios and native/android
  nimbus native icon [-background #hex] [-launch #hex] <image>   app icons and launch screens
  nimbus native build ios|android -release -server URL [-version V -build N] [-team ID] [-export app-store|ad-hoc|development]
  nimbus native keystore                               the Android release key
  nimbus native font add [-name Family] <font.ttf>     a font for Style.FontFamily
  nimbus native upgrade [version]                      move the project to the latest (or that) release
  nimbus native doctor
  nimbus native gen [app-dir]
  nimbus native build ios|android [-device [-udid PHONE] [-team ID]] [-server URL] [-abi list] [-name N] [-id ID] [-v] [app-dir]
  nimbus native run   ios|android [...same flags] [app-dir]
  nimbus native dev   ios|android [-sim|-device -udid D -team ID] [-no-server] [app-dir]   app + server, reload on save
  nimbus native lib   ios|android [flags] [project-dir]   the Go side of a project build (Xcode and Gradle run it)`)
	os.Exit(2)
}

// reorder puts flags before the arguments, so `new myapp -id com.me.app`
// works as well as `new -id com.me.app myapp`.
func reorder(args []string) []string {
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && !boolFlag(a) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		rest = append(rest, a)
	}
	return append(flags, rest...)
}

func boolFlag(a string) bool {
	switch strings.TrimLeft(a, "-") {
	case "v", "device", "sim", "release", "no-server", "quiet":
		return true
	}
	return false
}

// inModule refuses to go on outside a Go module, with what to do instead.
func inModule(dir string) error {
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if gomod := strings.TrimSpace(string(out)); err == nil && gomod != "" && gomod != os.DevNull {
		return nil
	}
	abs, _ := filepath.Abs(dir)
	return fmt.Errorf("%s is not a Nimbus Native project (no go.mod here or above).\n"+
		"  cd into a project made by `nimbus native new`, or make one:  nimbus native new myapp && cd myapp", abs)
}

// pickPhone decides where an iOS run goes: a phone when one is reachable
// (asking which when several are), else the simulator. build stays on the
// simulator unless -device.
func pickPhone(cmd string, device, sim bool, udid string) (toolchain.IOSPhone, bool, error) {
	if sim {
		return toolchain.IOSPhone{}, false, nil
	}
	if udid != "" {
		p, err := toolchain.PickIOSPhone(udid)
		return p, err == nil, err
	}
	if cmd != "run" && !device {
		return toolchain.IOSPhone{}, false, nil
	}
	phones, err := toolchain.IOSPhones()
	switch {
	case err != nil && device:
		return toolchain.IOSPhone{}, false, err
	case len(phones) == 0 && device:
		_, err := toolchain.PickIOSPhone("") // says how to connect one
		return toolchain.IOSPhone{}, false, err
	case len(phones) == 0:
		toolchain.Infof(os.Stdout, "No iPhone or iPad connected: using the simulator")
		return toolchain.IOSPhone{}, false, nil
	case len(phones) == 1:
		return phones[0], true, nil
	}
	names := make([]string, len(phones))
	for i, p := range phones {
		via := "USB"
		if p.Wifi {
			via = "Wi-Fi"
		}
		names[i] = fmt.Sprintf("%s (%s)", p.Name, via)
	}
	if !device {
		names = append(names, "iOS Simulator")
	}
	i, err := choose("Run on which device?", names, "-udid <name>")
	if err != nil || i == len(phones) {
		return toolchain.IOSPhone{}, false, err
	}
	return phones[i], true, nil
}

// pickTeam is the team that signs a phone build; asked for (and saved in
// the Xcode project) when Xcode has several and none is chosen.
func pickTeam(root, team string) (string, error) {
	t, err := toolchain.IOSTeam(root, team)
	if err == nil {
		return t, nil
	}
	teams := toolchain.XcodeTeams()
	if len(teams) < 2 || !interactive() {
		return "", err
	}
	var ids, names []string
	for id := range teams {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		names = append(names, fmt.Sprintf("%s (%s)", teams[id], id))
	}
	i, err := choose("Sign with which Apple team?", names, "-team <ID>")
	if err != nil {
		return "", err
	}
	if err := toolchain.SaveIOSTeam(root, ids[i]); err == nil {
		toolchain.Successf(os.Stdout, "Saved team %s in native/ios/Runner.xcodeproj", ids[i])
	}
	return ids[i], nil
}

// pickAndroid decides where an Android run goes: a connected phone first,
// else a running emulator (asking which when several), else "" so the
// first emulator boots.
func pickAndroid(env *toolchain.Env, device, sim bool, udid string) (string, error) {
	devs, err := env.AndroidDevices()
	if err != nil {
		return "", err
	}
	if udid != "" {
		for _, d := range devs {
			if d.Serial == udid || strings.EqualFold(d.Model, udid) {
				return d.Serial, nil
			}
		}
		return "", fmt.Errorf("no connected Android device %q", udid)
	}
	var phones, emulators []toolchain.AndroidDevice
	for _, d := range devs {
		if d.Emulator {
			emulators = append(emulators, d)
		} else {
			phones = append(phones, d)
		}
	}
	options := phones
	if sim || len(phones) == 0 {
		if device {
			return "", fmt.Errorf("no Android phone connected: plug one in with USB debugging on, or pair it for wireless debugging")
		}
		options = emulators
	}
	switch len(options) {
	case 0:
		return "", nil // the first emulator boots
	case 1:
		return options[0].Serial, nil
	}
	names := make([]string, len(options))
	for i, d := range options {
		kind := "phone"
		if d.Emulator {
			kind = "emulator"
		}
		names[i] = fmt.Sprintf("%s (%s, %s)", d.Model, kind, d.Serial)
	}
	i, err := choose("Run on which device?", names, "-udid <serial>")
	if err != nil {
		return "", err
	}
	return options[i].Serial, nil
}

// choose asks which of options to use, in a terminal; elsewhere it says to
// pick with a flag instead.
func choose(question string, options []string, flag string) (int, error) {
	if !interactive() {
		return 0, fmt.Errorf("%s pick one with %s:\n  %s", question, flag, strings.Join(options, "\n  "))
	}
	return toolchain.Choose(question, options)
}

// interactive reports whether someone at a terminal can answer.
func interactive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull) // a character device too, but nobody to ask
	return err != nil || !os.SameFile(fi, null)
}
