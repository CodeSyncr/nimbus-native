package toolchain

import (
	"bufio"
	"fmt"
	"io"
	"net"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// devServer runs a project's Nimbus server (its root package main) beside
// the app in dev mode: the web app and the API the phone talks to. Every
// save rebuilds it; a failed build keeps the last good one running.
type devServer struct {
	d    *devSession
	port string

	restartMu sync.Mutex // one rebuild at a time
	started   bool       // the first start printed Nimbus's banner
	mu        sync.Mutex
	cmd       *exec.Cmd
	done      chan struct{}
}

// newDevServer is the project's server, or nil when there is none to run
// (not a project, or no main.go). When its port is taken, it takes the next
// free one.
func newDevServer(d *devSession) *devServer {
	root := d.a.Root
	if root == "" || !exists(filepath.Join(root, "main.go")) {
		return nil
	}
	want := serverPort(root)
	port := want
	if n, err := strconv.Atoi(want); err == nil {
		for i := 0; i < 20 && taken(port); i++ {
			port = strconv.Itoa(n + i + 1)
		}
	}
	if port != want {
		d.con.say(warn, "⚠ Port %s is in use, so the server runs on %s.", want, port)
	}
	return &devServer{d: d, port: port}
}

func taken(port string) bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// serverPort is the port the server listens on: PORT from the environment
// or the project's .env, else Nimbus's default.
func serverPort(root string) string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	if f, err := os.Open(filepath.Join(root, ".env")); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "PORT="); ok {
				return strings.Trim(v, `"' `)
			}
		}
	}
	return "3333"
}

// serverEnv carries the server's address: at run time in dev mode, and
// into the app when set during a build (api.ServerURL).
const serverEnv = "NIMBUS_SERVER_URL"

// SetServer has this process's builds use url as the app's server.
func SetServer(url string) error {
	u, err := neturl.Parse(url)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.ContainsAny(url, " \t\"'") {
		return fmt.Errorf("-server %q: use an address like https://api.example.com", url)
	}
	return os.Setenv(serverEnv, strings.TrimRight(url, "/"))
}

// ServerSet reports whether an address was given for this build.
func ServerSet() bool { return os.Getenv(serverEnv) != "" }

// HasServer reports whether the project has a Nimbus server of its own.
func HasServer(root string) bool { return exists(filepath.Join(root, "main.go")) }

// Where a built app runs, for LocalServer.
const (
	OnSimulator = iota // shares this Mac's network
	OnEmulator         // reaches this Mac at 10.0.2.2
	OnPhone            // reaches this Mac over the local network
)

// LocalServer is where an app on target reaches the project's server on
// this Mac, or "" when this Mac isn't on a network.
func LocalServer(root string, target int) string {
	host := "127.0.0.1"
	switch target {
	case OnEmulator:
		host = "10.0.2.2"
	case OnPhone:
		if host = lanIP(); host == "" {
			return ""
		}
	}
	return "http://" + net.JoinHostPort(host, serverPort(root))
}

// lanIP is this Mac's address on the local network.
func lanIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsPrivate() {
			return n.IP.String()
		}
	}
	return ""
}

func (s *devServer) url() string { return "http://127.0.0.1:" + s.port }

// restart builds the server and replaces the running one.
func (s *devServer) restart() {
	s.restartMu.Lock()
	defer s.restartMu.Unlock()
	d := s.d
	bin := filepath.Join(d.a.BuildDir("dev"), "server")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = d.a.Root
	if out, err := build.CombinedOutput(); err != nil {
		d.con.say(bad, "✖ Server build failed:\n%s", indent(string(out)))
		d.con.say(warn, "⚠ Still running the last good server\n")
		return
	}
	s.stop()
	cmd := exec.Command(bin)
	cmd.Dir = d.a.Root
	cmd.Env = append(os.Environ(), "PORT="+s.port)
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		d.con.say(bad, "✖ Server: %v", err)
		return
	}
	logged := make(chan struct{})
	if s.started {
		// Nimbus prints a banner on every start: after the first, hold its
		// lines back until it is ready (and show them if it never gets there).
		go func() { s.quietStart(out); close(logged) }()
	} else {
		s.started = true
		go func() { d.con.stream(out, "server"); close(logged) }()
	}
	done := make(chan struct{})
	s.mu.Lock()
	s.cmd, s.done = cmd, done
	s.mu.Unlock()
	go func() {
		err := cmd.Wait()
		<-logged
		s.mu.Lock()
		ours := s.cmd == cmd // stop clears it before killing
		if ours {
			s.cmd = nil
		}
		s.mu.Unlock()
		close(done)
		if ours {
			d.con.say(bad, "\n✖ The server stopped (%v). Save to start it again.\n", err)
		}
	}()
}

// quietStart streams a restarted server's output, holding the start-up
// lines until Nimbus says it is ready, then one line in their place.
func (s *devServer) quietStart(r io.Reader) {
	pr, pw := io.Pipe()
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		var held []string
		ready := false
		for sc.Scan() {
			line := sc.Text()
			if ready {
				fmt.Fprintln(pw, line)
				continue
			}
			held = append(held, line)
			if strings.Contains(line, "Ready in") {
				ready = true
				s.d.con.say(good, "✓ Server restarted on %s", s.url())
			}
		}
		if !ready {
			for _, l := range held {
				fmt.Fprintln(pw, l)
			}
		}
		pw.Close()
	}()
	s.d.con.stream(pr, "server")
}

func (s *devServer) stop() {
	s.mu.Lock()
	cmd, done := s.cmd, s.done
	s.cmd = nil
	s.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		<-done
	}
}

// open shows the web app in the default browser.
func (s *devServer) open() {
	if err := exec.Command("open", s.url()).Start(); err != nil {
		s.d.con.say(bad, "✖ %v", fmt.Errorf("opening %s: %w", s.url(), err))
	}
}
