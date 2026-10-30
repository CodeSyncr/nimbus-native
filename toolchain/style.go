package toolchain

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"
)

// The nimbus CLI's look: the same marks, colors, panels and arrow-key
// pickers as the commands of nimbus itself, with the standard library alone
// so that this module keeps no dependencies.

// Terminal colors, as nimbus uses them.
const (
	colorSuccess = "38;5;10"
	colorWarn    = "38;5;220"
	colorError   = "1;38;5;196"
	colorInfo    = "38;5;39"
	colorBold    = "1"
	colorDim     = "2"
)

// styled reports whether w is a terminal that wants color.
func styled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func paint(w io.Writer, code, s string) string {
	if code == "" || !styled(w) {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

// Successf writes a line that something worked.
func Successf(w io.Writer, format string, args ...any) {
	mark(w, colorSuccess, "✓ ", format, args...)
}

// Warnf writes a line that something needs attention.
func Warnf(w io.Writer, format string, args ...any) { mark(w, colorWarn, "⚠ ", format, args...) }

// Infof writes a line of what is happening.
func Infof(w io.Writer, format string, args ...any) { mark(w, colorInfo, "ℹ ", format, args...) }

// Errorf writes a failure. Only its first line is colored: the rest is
// usually a tool's own output, there to be read.
func Errorf(w io.Writer, format string, args ...any) { mark(w, colorError, "✖ ", format, args...) }

func mark(w io.Writer, code, sign, format string, args ...any) {
	first, rest, more := strings.Cut(fmt.Sprintf(format, args...), "\n")
	fmt.Fprintln(w, paint(w, code, sign+first))
	if more {
		fmt.Fprintln(w, rest)
	}
}

// Panel writes a title and body in a rounded box.
func Panel(w io.Writer, title, body string) {
	lines := append([]string{"", title, ""}, strings.Split(body, "\n")...)
	lines = append(lines, "")
	width := 0
	for _, l := range lines {
		width = max(width, utf8.RuneCountInString(l))
	}
	rule := strings.Repeat("─", width+4)
	fmt.Fprintln(w, "╭"+rule+"╮")
	for i, l := range lines {
		pad := strings.Repeat(" ", width-utf8.RuneCountInString(l))
		if i == 1 {
			l = paint(w, colorBold, l)
		}
		fmt.Fprintln(w, "│  "+l+pad+"  │")
	}
	fmt.Fprintln(w, "╰"+rule+"╯")
}

// pickKey moves a picker of n options from i for one key press (as read
// from the terminal, so an arrow is several bytes); done means chosen,
// cancel means backed out.
func pickKey(i, n int, key []byte) (next int, done, cancel bool) {
	switch k := string(key); k {
	case "\r", "\n":
		return i, true, false
	case "\x03", "\x1b":
		return i, false, true
	case "\x1b[A", "\x1bOA", "k":
		return (i + n - 1) % n, false, false
	case "\x1b[B", "\x1bOB", "j":
		return (i + 1) % n, false, false
	default:
		if len(k) == 1 && k[0] >= '1' && int(k[0]-'0') <= n {
			return int(k[0] - '1'), false, false
		}
	}
	return i, false, false
}

// Choose asks which of options to use, with the arrow keys. It needs a
// terminal on stdin; the answer stays on screen.
func Choose(question string, options []string) (int, error) {
	out := os.Stdout
	stty := func(args ...string) error {
		cmd := exec.Command("stty", args...)
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}
	if err := stty("-icanon", "-echo", "-isig", "min", "1"); err != nil {
		return 0, fmt.Errorf("reading the keyboard: %w", err)
	}
	defer stty("icanon", "echo", "isig")
	fmt.Fprint(out, "\033[?25l") // no cursor while choosing
	defer fmt.Fprint(out, "\033[?25h")

	prompt := paint(out, "1;38;5;10", "? ") + paint(out, colorBold, question)
	draw := func(i int) {
		fmt.Fprintln(out, prompt+"  "+paint(out, colorDim, "[Use arrows to move, enter to select]"))
		for j, o := range options {
			if j == i {
				fmt.Fprintln(out, paint(out, colorInfo, "> "+o))
			} else {
				fmt.Fprintln(out, "  "+o)
			}
		}
	}
	erase := func() { fmt.Fprintf(out, "\033[%dA\033[J", len(options)+1) }

	i := 0
	draw(i)
	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return 0, errors.New("no choice made")
		}
		next, done, cancel := pickKey(i, len(options), buf[:n])
		if cancel {
			return 0, errors.New("cancelled")
		}
		i = next
		erase()
		if done {
			fmt.Fprintln(out, prompt+" "+paint(out, colorInfo, options[i]))
			return i, nil
		}
		draw(i)
	}
}
