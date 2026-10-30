package toolchain

import (
	"bytes"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/CodeSyncr/nimbus-native/dev/frame"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Dev runs the app's Go code on this machine against a dev shell on the
// simulator or emulator, and restarts it on every save:
//
//	shell (device) ⇄ relay (here) ⇄ app process (here, rebuilt on change)
//
// The shell is the real app with its Go library replaced by a proxy
// (dev/client), so the native side is exactly what ships.
func (e *Env) Dev(a *App, platform string, port int, target ...DevTarget) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listen, shellAddr := addr, addr
	var t DevTarget
	if len(target) > 0 {
		t = target[0]
	}
	if t.Phone != nil {
		// The phone reaches the relay over the network, at this Mac's address.
		ip, err := lanIPv4()
		if err != nil {
			return err
		}
		listen, shellAddr = fmt.Sprintf(":%d", port), fmt.Sprintf("%s:%d", ip, port)
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("dev relay: %w (is another `nimbus native dev` running?)", err)
	}
	defer ln.Close()

	con := newConsole(e.Out)
	d := &devSession{e: e, a: a, platform: platform, addr: addr, shellAddr: shellAddr, target: t, con: con}
	d.relay = &relay{con: con, onDevice: d.deviceConnected}
	go d.relay.serve(ln)

	// Each session starts on the first screen; reloads within it keep the
	// screen (ui/navstate.go).
	os.Remove(d.navFile())

	// The app process starts when the shell connects (deviceConnected).
	con.say(plain, "\n  Launching %s on %s…", a.Name, d.where())
	if !t.NoServer {
		d.server = newDevServer(d)
	}
	if err := d.buildShell(); err != nil {
		return err
	}
	if d.server != nil {
		if platform == "android" {
			// The app reaches the server at the device's own 127.0.0.1.
			_ = e.run(nil, e.adbPath(), e.adbArgs("reverse", "tcp:"+d.server.port, "tcp:"+d.server.port)...)
		}
		go d.server.restart()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	presses, restore := keys()
	defer restore()
	snap := snapshot(a.root())
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	quit := func() error {
		if d.server != nil {
			d.server.stop()
		}
		d.stopApp()
		d.stopDeviceLogs()
		restore()
		con.say(plain, "\n  Stopped.")
		return nil
	}
	reload := func(why string) {
		con.say(dim, "\n  %s…", why)
		if err := d.reload(time.Now()); err != nil {
			con.say(bad, "%v", err)
			con.say(warn, "  (still running the last good build)\n")
		} else if d.server != nil {
			go d.server.restart()
		}
		snap = snapshot(a.root())
	}
	for {
		select {
		case <-stop:
			return quit()
		case k := <-presses:
			switch k {
			case 'r':
				reload("Reloading")
			case 'R':
				con.say(dim, "\n  Restarting: rebuilding and reinstalling the app…")
				d.stopApp()
				t0 := time.Now()
				if err := d.buildShell(); err != nil {
					con.say(bad, "%v", err)
				} else {
					con.say(dim, "  Reinstalled in %.1fs.", time.Since(t0).Seconds())
				}
				snap = snapshot(a.root())
			case 'c':
				con.clear()
			case 'o':
				if d.server != nil {
					d.server.open()
				}
			case 'h', '?':
				d.help()
			case 'q', 'Q':
				return quit()
			}
		case <-tick.C:
			if now := snapshot(a.root()); now != snap {
				reload("Change saved: reloading")
			}
		}
	}
}

func (d *devSession) help() {
	web := ""
	if d.server != nil {
		web = d.server.url()
	}
	d.con.help(d.a.Name, d.where(), web)
}

// where names the device dev mode runs on.
func (d *devSession) where() string {
	switch {
	case d.target.Phone != nil:
		return d.target.Phone.Name
	case d.platform == "ios":
		return "the iOS simulator"
	case d.e.Serial != "":
		return d.e.Serial
	}
	return "Android"
}

// DevTarget is where dev mode runs its shell: the simulator or the device
// Env.Serial names by default, or an iPhone or iPad.
type DevTarget struct {
	Phone *IOSPhone
	Team  string // signs the phone build
	// NoServer leaves a project's Nimbus server (go run .) out of dev mode.
	NoServer bool
}

type devSession struct {
	e         *Env
	a         *App
	platform  string
	addr      string // the relay, for the app on this machine
	shellAddr string // the relay, for the shell on the device
	target    DevTarget
	relay     *relay
	con       *console
	server    *devServer // nil: no project server to run

	shellSchema uint64 // combined hash the installed shell was built with

	restartMu sync.Mutex // one rebuild at a time (a save and a reconnect can coincide)
	appMu     sync.Mutex
	app       *exec.Cmd
	appDone   chan struct{} // closed when app exits
	connected bool          // the shell has connected once

	logs *exec.Cmd
}

// shellApp is the app as the dev shell builds it: same name, id and host
// code, with the proxy as its Go library.
func (d *devSession) shellApp() *App {
	s := *d.a
	s.GoDir = filepath.Join(d.a.root(), "build", "devshell")
	return &s
}

// buildShell generates the proxy package, builds and installs the shell, and
// launches it with its logs streaming here.
func (d *devSession) buildShell() error {
	e, shell := d.e, d.shellApp()
	if err := os.MkdirAll(shell.GoDir, 0o755); err != nil {
		return err
	}
	// The proxy package must exist before gen can list its dependencies;
	// then it is rewritten with the schema hashes gen found.
	if err := writeShellMain(shell.GoDir, d.shellAddr, 0, nil); err != nil {
		return err
	}
	g, err := e.Gen(shell)
	if err != nil {
		return err
	}
	if err := writeShellMain(shell.GoDir, d.shellAddr, g.AppCombined, g.Hashes); err != nil {
		return err
	}
	d.shellSchema = g.Combined
	d.relay.setExpected(g.AppCombined)

	e.step("dev shell for %s", d.platform)
	d.stopDeviceLogs()
	switch {
	case d.platform == "ios" && d.target.Phone != nil:
		if shell.Root == "" {
			return fmt.Errorf("dev mode on a phone needs a project made by `nimbus native new` (use -sim here)")
		}
		p := *d.target.Phone
		app, err := e.BuildIOSProject(shell, IOSDevice, IOSDeviceBuild{Team: d.target.Team, Phone: p,
			// iOS asks once before an app talks to the local network: here, to this Mac.
			Settings: []string{"INFOPLIST_KEY_NSLocalNetworkUsageDescription=Dev mode: the app talks to nimbus native dev on your Mac."}})
		if err != nil {
			return err
		}
		if err := e.installOnPhone(app, p); err != nil {
			return err
		}
		if err := e.openOnPhone(d.a.ID, p, false); err != nil {
			return err
		}
		d.con.say(warn, "  On %s, allow the app to find devices on your local network if asked.", p.Name)
		return nil // the app's output comes from its process here
	case d.platform == "ios":
		build := e.BuildIOS
		if shell.Root != "" {
			// the project's Xcode build, with the proxy as its Go
			build = func(a *App, t IOSTarget) (string, error) { return e.BuildIOSProject(a, t) }
		}
		app, err := build(shell, IOSSimulator)
		if err != nil {
			return err
		}
		udid, err := bootedSimulator()
		if err != nil {
			return err
		}
		if err := e.run(nil, "xcrun", "simctl", "install", udid, app); err != nil {
			return err
		}
		e.step("launch %s on the simulator", d.a.ID)
		d.logs = exec.Command("xcrun", "simctl", "launch", "--console-pty", "--terminate-running-process", udid, d.a.ID)
	case d.platform == "android":
		build := e.BuildAndroid
		if shell.Root != "" {
			build = e.BuildAndroidProject
		}
		apk, err := build(shell, []string{"arm64-v8a"})
		if err != nil {
			return err
		}
		if err := e.ensureDevice(); err != nil {
			return err
		}
		port := d.addr[strings.LastIndex(d.addr, ":")+1:]
		// The device reaches this machine's relay at its own 127.0.0.1.
		if err := e.run(nil, e.adbPath(), e.adbArgs("reverse", "tcp:"+port, "tcp:"+port)...); err != nil {
			return err
		}
		if err := e.run(nil, e.adbPath(), e.adbArgs("install", "-r", "-t", "--no-incremental", apk)...); err != nil {
			return err
		}
		_ = e.adb("logcat", "-c").Run()
		e.step("launch %s", d.a.ID)
		if err := e.run(nil, e.adbPath(), e.adbArgs("shell", "am", "start", "-S", "-n", d.a.ID+"/.MainActivity")...); err != nil {
			return err
		}
		d.logs = e.adb("logcat", "-v", "brief", "-s", "NimbusNative:*", "NimbusNativeApp:*", "AndroidRuntime:E")
	default:
		return fmt.Errorf("unknown platform %q (ios or android)", d.platform)
	}
	return d.streamDeviceLogs()
}

func (d *devSession) streamDeviceLogs() error {
	out, err := d.logs.StdoutPipe()
	if err != nil {
		return err
	}
	d.logs.Stderr = d.logs.Stdout
	if err := d.logs.Start(); err != nil {
		return err
	}
	go d.con.stream(out, "device")
	return nil
}

func (d *devSession) stopDeviceLogs() {
	if d.logs != nil && d.logs.Process != nil {
		d.logs.Process.Kill()
		d.logs.Wait()
	}
	d.logs = nil
}

// reload regenerates, rebuilds and restarts after a change; a schema change
// also rebuilds and reinstalls the shell.
func (d *devSession) reload(t0 time.Time) error {
	g, err := d.e.Gen(d.shellApp())
	if err != nil {
		return err
	}
	if g.Combined != d.shellSchema {
		d.con.say(warn, "  The wire schema changed: rebuilding and reinstalling the app.")
		d.stopApp()
		if err := d.buildShell(); err != nil {
			return err
		}
	}
	return d.restartApp(t0)
}

// restartApp builds the app for this machine and replaces the running
// process. A failed build leaves the old process running.
func (d *devSession) restartApp(t0 time.Time) error {
	d.restartMu.Lock()
	defer d.restartMu.Unlock()
	bin := filepath.Join(d.a.BuildDir("dev"), "app")
	build := exec.Command("go", "build", "-tags", "nimbusdev", "-o", bin, ".")
	build.Dir = d.a.Dir
	var buf bytes.Buffer
	build.Stdout, build.Stderr = &buf, &buf
	if err := build.Run(); err != nil {
		return fmt.Errorf("  Build failed:\n%s", indent(buf.String()))
	}
	built := time.Since(t0)

	d.stopApp()
	cmd := exec.Command(bin)
	cmd.Dir = d.a.Dir
	cmd.Env = append(os.Environ(), "NIMBUS_DEV_ADDR="+d.addr, "NIMBUS_DEV_STATE="+d.navFile())
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	d.relay.expectReload(t0, built)
	if err := cmd.Start(); err != nil {
		return err
	}
	logged := make(chan struct{})
	go func() { d.con.stream(out, "app   "); close(logged) }()
	done := make(chan struct{})
	d.appMu.Lock()
	d.app, d.appDone = cmd, done
	d.appMu.Unlock()
	go func() {
		err := cmd.Wait()
		<-logged
		d.appMu.Lock()
		ours := d.app == cmd // stopApp clears it before killing
		if ours {
			d.app = nil
		}
		d.appMu.Unlock()
		close(done)
		if ours {
			d.con.say(bad, "\n  The app stopped (%v). Fix the code and save to start it again, or press r.\n", err)
		}
	}()
	return nil
}

// navFile is where the app saves its tabs and stacks for the next reload.
func (d *devSession) navFile() string { return filepath.Join(d.a.BuildDir("dev"), "navstate") }

func (d *devSession) stopApp() {
	d.appMu.Lock()
	app, done := d.app, d.appDone
	d.app = nil
	d.appMu.Unlock()
	if app != nil && app.Process != nil {
		app.Process.Kill()
		<-done
	}
}

// deviceConnected restarts the app process: the shell's views are new, so
// the app's idea of what is on screen must be too.
func (d *devSession) deviceConnected() {
	d.appMu.Lock()
	first := !d.connected
	d.connected = true
	d.appMu.Unlock()
	if first {
		d.help()
	} else {
		d.con.say(dim, "  The app reconnected.")
	}
	go func() {
		if err := d.restartApp(time.Now()); err != nil {
			d.con.say(bad, "%v", err)
		}
	}()
}

func writeShellMain(dir, addr string, appSchema uint64, hashes []uint64) error {
	var b strings.Builder
	b.WriteString("// Code generated by nimbus native dev. DO NOT EDIT.\n\n")
	b.WriteString("// The dev shell's Go library: a proxy to the app running on the laptop.\n")
	b.WriteString("package main\n\nimport (\n\t\"github.com/CodeSyncr/nimbus-native/dev/client\"\n\t\"github.com/CodeSyncr/nimbus-native/wire\"\n)\n\n")
	b.WriteString("func init() {\n")
	fmt.Fprintf(&b, "\tclient.Addr = %q\n\tclient.AppSchema = %#016x\n", addr, appSchema)
	b.WriteString("\t// The app's schemas, so the shell's hash matches its generated host code.\n")
	for _, h := range hashes {
		fmt.Fprintf(&b, "\twire.Register(\"app\", %#016x)\n", h)
	}
	b.WriteString("}\n\nfunc main() {}\n")
	return writeIfChanged(filepath.Join(dir, "main.go"), []byte(b.String()))
}

// snapshot fingerprints the app's Go sources (not generated files, build
// output or a project's native/ folder), so a poll can tell when something
// was saved. Generated files are left out so the reload that rewrites them
// doesn't trigger another.
func snapshot(root string) string {
	var b strings.Builder
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (name == "build" || name == "native" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if (strings.HasSuffix(name, ".go") && name != "wire_gen.go" && name != RoutesFile) || name == "go.mod" || name == "go.sum" {
			if info, err := d.Info(); err == nil {
				fmt.Fprintf(&b, "%s %d %d\n", p, info.Size(), info.ModTime().UnixNano())
			}
		}
		return nil
	})
	return b.String()
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n    ")
}

// relay pairs the device's proxy with the current app process and forwards
// frames between them.
type relay struct {
	con      *console
	onDevice func()

	mu          sync.Mutex
	device, app *peer
	expected    uint64

	// Host-call answers (text measurements) cached across app restarts:
	// a reload re-measures every text, and the device's answer for the same
	// request doesn't change while it stays connected.
	answers     map[string][]byte
	asked       map[uint32][]byte // in-flight host calls of the current app, by id
	hits        int
	reloadStart time.Time
	buildTime   time.Duration
	awaitFirst  bool
}

type peer struct {
	c  net.Conn
	mu sync.Mutex
}

func (p *peer) write(t frame.Type, b []byte) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	frame.Write(p.c, t, b)
}

func (r *relay) setExpected(h uint64) {
	r.mu.Lock()
	r.expected = h
	r.mu.Unlock()
}

func (r *relay) expectReload(t0 time.Time, built time.Duration) {
	r.mu.Lock()
	r.reloadStart, r.buildTime, r.awaitFirst = t0, built, true
	r.mu.Unlock()
}

func (r *relay) serve(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go r.handle(c)
	}
}

func (r *relay) handle(c net.Conn) {
	t, p, err := frame.Read(c)
	role, schema, ok := frame.ParseHello(p)
	if err != nil || t != frame.Hello || !ok {
		c.Close()
		return
	}
	me := &peer{c: c}
	r.mu.Lock()
	switch role {
	case frame.RoleDevice:
		if r.device != nil {
			r.device.c.Close()
		}
		r.device = me
		r.answers = map[string][]byte{} // new device, new fonts and settings
		r.mu.Unlock()
		r.onDevice()
	case frame.RoleApp:
		if r.app != nil {
			r.app.c.Close()
		}
		r.app = me
		r.asked, r.hits = map[uint32][]byte{}, 0
		dev, expected := r.device, r.expected
		r.mu.Unlock()
		if schema != expected {
			r.con.say(bad, "  The app's wire schema (%#x) differs from the dev shell's (%#x); restart nimbus native dev.", schema, expected)
		}
		dev.write(frame.Reset, nil) // clear the old views; the proxy replays the viewport
	default:
		r.mu.Unlock()
		c.Close()
		return
	}

	for {
		t, p, err := frame.Read(c)
		if err != nil {
			r.mu.Lock()
			if r.device == me {
				r.device = nil
			}
			if r.app == me {
				r.app = nil
			}
			r.mu.Unlock()
			c.Close()
			return
		}
		if role == frame.RoleApp && t == frame.HostCall && r.answerFromCache(me, p) {
			continue
		}
		if role == frame.RoleDevice && t == frame.HostReply {
			r.remember(p)
		}
		r.mu.Lock()
		dev, app := r.device, r.app
		first := role == frame.RoleApp && t == frame.FromGo && r.awaitFirst
		if first {
			r.awaitFirst = false
		}
		start, built := r.reloadStart, r.buildTime
		r.mu.Unlock()
		if role == frame.RoleDevice {
			app.write(t, p)
		} else {
			dev.write(t, p)
			if first {
				r.mu.Lock()
				hits := r.hits
				r.mu.Unlock()
				r.con.say(good, "  ↻ On screen in %.2fs (build %.2fs, %d measurements from cache)",
					time.Since(start).Seconds(), built.Seconds(), hits)
			}
		}
	}
}

// answerFromCache replies to an app's host call from the cache, or records
// it so the device's reply can be cached. It reports whether it replied.
func (r *relay) answerFromCache(app *peer, p []byte) bool {
	id, req, ok := frame.SplitID(p)
	if !ok {
		return false
	}
	// Only text measurements repeat exactly and answer the same each time;
	// anything else (plugin calls) always goes to the device.
	if m, err := render.Decode(req); err != nil {
		return false
	} else if _, measure := m.(*render.MeasureText); !measure {
		return false
	}
	r.mu.Lock()
	reply, hit := r.answers[string(req)]
	if hit {
		r.hits++
	} else if r.asked != nil {
		r.asked[id] = req
	}
	r.mu.Unlock()
	if hit {
		app.write(frame.HostReply, frame.Reply(id, 0, reply))
	}
	return hit
}

// remember caches a successful device reply to a host call.
func (r *relay) remember(p []byte) {
	id, rc, reply, ok := frame.SplitReply(p)
	if !ok {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if req, ok := r.asked[id]; ok {
		delete(r.asked, id)
		if rc == 0 && r.answers != nil {
			r.answers[string(req)] = reply
		}
	}
}

// lanIPv4 is this machine's address on the local network: where a phone on
// the same Wi-Fi reaches it.
func lanIPv4() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	var found []string
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && ipn.IP.IsPrivate() {
				if ifc.Name == "en0" {
					return ipn.IP.String(), nil // Wi-Fi on most Macs
				}
				found = append(found, ipn.IP.String())
			}
		}
	}
	if len(found) == 0 {
		return "", fmt.Errorf("this Mac has no local network address for the phone to reach: join the phone's Wi-Fi")
	}
	return found[0], nil
}
