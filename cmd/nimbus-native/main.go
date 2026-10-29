// Command nimbus-native builds and runs Nimbus Native apps. It will become
// `nimbus native …` in the Nimbus CLI; it lives here while the toolchain is
// built.
//
//	nimbus-native doctor
//	nimbus-native gen [app-dir]
//	nimbus-native build ios|android [flags] [app-dir]
//	nimbus-native run   ios|android [flags] [app-dir]
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

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  nimbus-native doctor
  nimbus-native gen [app-dir]
  nimbus-native build ios|android [-device] [-abi list] [-name N] [-id ID] [-v] [app-dir]
  nimbus-native run   ios|android [...same flags] [app-dir]`)
	os.Exit(2)
}
