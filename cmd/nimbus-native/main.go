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
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
	device := fs.Bool("device", false, "build for (and run on) a phone; run picks one by default when connected")
	sim := fs.Bool("sim", false, "run on the simulator / emulator even when a phone is connected")
	udid := fs.String("udid", "", "which phone (or emulator), by name, id or serial")
	team := fs.String("team", "", "iOS: the Apple team that signs device builds (or NIMBUS_IOS_TEAM)")
	abis := fs.String("abi", "arm64-v8a", "Android: comma-separated ABIs (arm64-v8a,armeabi-v7a,x86_64,x86)")
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}

	if err := inModule(dir); err != nil {
		return err
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
			out, err := env.BuildIOSProject(app, toolchain.IOSSimulator)
			if err != nil {
				return err
			}
			fmt.Println("built", out)
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
		fmt.Printf("› %s over %s, signed by team %s\n", phone.Name, via, team)
		out, err := env.BuildIOSProject(app, toolchain.IOSDevice, toolchain.IOSDeviceBuild{Team: team, Phone: phone})
		if err != nil {
			return err
		}
		fmt.Println("built", out)
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
	template := fs.String("template", "counter", "the app to start from: "+strings.Join(toolchain.Templates, " or "))
	verbose := fs.Bool("v", false, "print every tool invocation")
	_ = fs.Parse(reorder(args))
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: nimbus native new [-template counter|shop] [-name N] [-module M] [-id ID] <dir>")
	}
	p, err := toolchain.NewProjectFor(fs.Arg(0), *name, *module, *id)
	if err != nil {
		return err
	}
	p.Template = *template
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
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	port := fs.Int("port", 7357, "port for the dev relay")
	device := fs.Bool("device", false, "use a phone (the default when one is connected)")
	sim := fs.Bool("sim", false, "use the simulator / emulator even when a phone is connected")
	udid := fs.String("udid", "", "which phone (or emulator), by name, id or serial")
	team := fs.String("team", "", "iOS: the Apple team that signs the phone build")
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
	return env.Dev(app, platform, *port, target)
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  nimbus native new [-template counter|shop] <dir>    a project with native/ios and native/android
  nimbus native doctor
  nimbus native gen [app-dir]
  nimbus native build ios|android [-device [-udid PHONE] [-team ID]] [-abi list] [-name N] [-id ID] [-v] [app-dir]
  nimbus native run   ios|android [...same flags] [app-dir]
  nimbus native dev   ios|android [-sim|-device -udid D -team ID] [-port N] [app-dir]   reload on save
  nimbus native lib   ios|android [flags] [project-dir]   the Go side of a project build (Xcode and Gradle run it)`)
	os.Exit(2)
}

// reorder puts flags before the arguments, so `new shop -template shop`
// works as well as `new -template shop shop`.
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

func boolFlag(a string) bool { return a == "-v" || a == "--v" || a == "-device" || a == "--device" }

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
		fmt.Println("› no iPhone or iPad connected: using the simulator")
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
		fmt.Printf("› saved team %s in native/ios/Runner.xcodeproj\n", ids[i])
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
	fmt.Printf("\n%s\n", question)
	for i, o := range options {
		fmt.Printf("  %d) %s\n", i+1, o)
	}
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Choose 1-%d: ", len(options))
		line, err := in.ReadString('\n')
		if n, convErr := strconv.Atoi(strings.TrimSpace(line)); convErr == nil && n >= 1 && n <= len(options) {
			fmt.Println()
			return n - 1, nil
		}
		if err != nil {
			return 0, fmt.Errorf("no choice made")
		}
	}
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
