package toolchain

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Dev mode's terminal: what is happening, in color when it is a terminal,
// the app's own output, and single-key commands.

type tone int

const (
	plain tone = iota
	dim
	good
	bad
	warn
)

// console writes dev mode's lines, one at a time.
type console struct {
	mu    sync.Mutex
	out   io.Writer
	color bool
}

func newConsole(out io.Writer) *console {
	c := &console{out: out}
	if f, ok := out.(*os.File); ok && os.Getenv("NO_COLOR") == "" {
		if fi, err := f.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
			c.color = true
		}
	}
	return c
}

func (c *console) paint(t tone, s string) string {
	if !c.color {
		return s
	}
	code := map[tone]string{dim: "2", good: "32", bad: "31", warn: "33"}[t]
	if code == "" {
		return s
	}
	// Blank lines around the text stay outside the color.
	body := strings.Trim(s, "\n")
	if body == "" {
		return s
	}
	i := strings.Index(s, body)
	return s[:i] + "\033[" + code + "m" + body + "\033[0m" + s[i+len(body):]
}

// say writes one line, in a tone.
func (c *console) say(t tone, format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fmt.Fprintln(c.out, c.paint(t, fmt.Sprintf(format, args...)))
}

// stream copies a process's output line by line under a dim label; panics
// and errors stand out in red.
func (c *console) stream(r io.Reader, label string) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	inPanic := false
	for sc.Scan() {
		line := sc.Text()
		low := strings.ToLower(line)
		if strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "fatal error:") {
			inPanic = true
		}
		t := plain
		if inPanic || strings.Contains(low, "error") {
			t = bad
		}
		c.mu.Lock()
		fmt.Fprintf(c.out, "%s %s\n", c.paint(dim, label+" │"), c.paint(t, line))
		c.mu.Unlock()
	}
}

func (c *console) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.color {
		fmt.Fprint(c.out, "\033[2J\033[H")
	}
}

func (c *console) help(app, target string) {
	c.say(good, "\n  ✓ %s is running on %s.", app, target)
	c.say(plain, "  Save a .go file to reload, or press a key:")
	c.say(dim, "    r  reload    R  restart (rebuild and reinstall)    c  clear    q  quit    h  help\n")
}

// keys reads single key presses from the terminal, if there is one, until
// stop is called. It returns nil when stdin isn't a terminal.
func keys() (presses <-chan byte, stop func()) {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return nil, func() {}
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return nil, func() {}
	}
	stty := func(args ...string) error {
		cmd := exec.Command("stty", args...)
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}
	if stty("-icanon", "-echo", "min", "1") != nil {
		return nil, func() {}
	}
	ch := make(chan byte, 8)
	go func() {
		b := make([]byte, 1)
		for {
			if n, err := os.Stdin.Read(b); err != nil || n == 0 {
				return
			}
			ch <- b[0]
		}
	}()
	var once sync.Once
	return ch, func() { once.Do(func() { stty("icanon", "echo") }) }
}
