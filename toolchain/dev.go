package toolchain

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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
func (e *Env) Dev(a *App, platform string, port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("dev relay: %w (is another `nimbus native dev` running?)", err)
	}
	defer ln.Close()

	d := &devSession{e: e, a: a, platform: platform, addr: addr}
	d.relay = &relay{out: e.Out, onDevice: d.deviceConnected}
	go d.relay.serve(ln)

	// Each session starts on the first screen; reloads within it keep the
	// screen (ui/navstate.go).
	os.Remove(d.navFile())

	// The app process starts when the shell connects (deviceConnected).
	if err := d.buildShell(); err != nil {
		return err
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	snap := snapshot(a.root())
	fmt.Fprintf(e.Out, "\n  Watching %s — save a .go file to reload. Ctrl+C to stop.\n\n", a.root())
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			d.stopApp()
			d.stopDeviceLogs()
			fmt.Fprintln(e.Out, "\n  Stopped.")
			return nil
		case <-tick.C:
			now := snapshot(a.root())
			if now == snap {
				continue
			}
			snap = now
			t0 := time.Now()
			if err := d.reload(t0); err != nil {
				fmt.Fprintf(e.Out, "\n%v\n  (still running the last good build)\n\n", err)
			}
			snap = snapshot(a.root())
		}
	}
}

type devSession struct {
	e        *Env
	a        *App
	platform string
	addr     string
	relay    *relay

	shellSchema uint64 // combined hash the installed shell was built with

	restartMu sync.Mutex // one rebuild at a time (a save and a reconnect can coincide)
	appMu     sync.Mutex
	app       *exec.Cmd

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
	if err := writeShellMain(shell.GoDir, d.addr, 0, nil); err != nil {
		return err
	}
	g, err := e.Gen(shell)
	if err != nil {
		return err
	}
	if err := writeShellMain(shell.GoDir, d.addr, g.AppCombined, g.Hashes); err != nil {
		return err
	}
	d.shellSchema = g.Combined
	d.relay.setExpected(g.AppCombined)

	e.step("dev shell for %s", d.platform)
	d.stopDeviceLogs()
	switch d.platform {
	case "ios":
		build := e.BuildIOS
		if shell.Root != "" {
			build = e.BuildIOSProject // the project's Xcode build, with the proxy as its Go
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
	case "android":
		build := e.BuildAndroid
		if shell.Root != "" {
			build = e.BuildAndroidProject
		}
		apk, err := build(shell, []string{"arm64-v8a"})
		if err != nil {
			return err
		}
		adb := filepath.Join(e.AndroidSDK, "platform-tools/adb")
		if err := e.ensureDevice(adb); err != nil {
			return err
		}
		port := d.addr[strings.LastIndex(d.addr, ":")+1:]
		// The device reaches this machine's relay at its own 127.0.0.1.
		if err := e.run(nil, adb, "reverse", "tcp:"+port, "tcp:"+port); err != nil {
			return err
		}
		if err := e.run(nil, adb, "install", "-r", "-t", "--no-incremental", apk); err != nil {
			return err
		}
		_ = exec.Command(adb, "logcat", "-c").Run()
		e.step("launch %s on the emulator", d.a.ID)
		if err := e.run(nil, adb, "shell", "am", "start", "-S", "-n", d.a.ID+"/.MainActivity"); err != nil {
			return err
		}
		d.logs = exec.Command(adb, "logcat", "-v", "brief", "-s", "NimbusNative:*", "NimbusNativeApp:*", "AndroidRuntime:E")
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
	go prefixLines(d.e.Out, out, "device │ ")
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
		fmt.Fprintln(d.e.Out, "  Wire schema changed: rebuilding the dev shell.")
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
	go prefixLines(d.e.Out, out, "app    │ ")
	d.appMu.Lock()
	d.app = cmd
	d.appMu.Unlock()
	return nil
}

// navFile is where the app saves its tabs and stacks for the next reload.
func (d *devSession) navFile() string { return filepath.Join(d.a.BuildDir("dev"), "navstate") }

func (d *devSession) stopApp() {
	d.appMu.Lock()
	defer d.appMu.Unlock()
	if d.app != nil && d.app.Process != nil {
		d.app.Process.Kill()
		d.app.Wait()
	}
	d.app = nil
}

// deviceConnected restarts the app process: the shell's views are new, so
// the app's idea of what is on screen must be too.
func (d *devSession) deviceConnected() {
	go func() {
		if err := d.restartApp(time.Now()); err != nil {
			fmt.Fprintln(d.e.Out, err)
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

func prefixLines(w io.Writer, r io.Reader, prefix string) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		fmt.Fprintf(w, "%s%s\n", prefix, sc.Text())
	}
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n    ")
}

// relay pairs the device's proxy with the current app process and forwards
// frames between them.
type relay struct {
	out      io.Writer
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
		fmt.Fprintln(r.out, "  Device connected.")
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
			fmt.Fprintf(r.out, "  The app's wire schema (%#x) differs from the dev shell's (%#x); restart nimbus native dev.\n", schema, expected)
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
				fmt.Fprintf(r.out, "  ↻ on screen %.2fs after the change (build %.2fs, %d measurements from cache)\n",
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
