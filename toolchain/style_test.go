package toolchain

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMarks(t *testing.T) {
	var b bytes.Buffer
	Successf(&b, "built %s", "a")
	Warnf(&b, "careful")
	Infof(&b, "working")
	Errorf(&b, "failed\ntool output")
	want := "✓ built a\n⚠ careful\nℹ working\n✖ failed\ntool output\n"
	if b.String() != want {
		t.Fatalf("got %q", b.String())
	}
}

func TestPanelLinesLineUp(t *testing.T) {
	var b bytes.Buffer
	Panel(&b, "Test — running", "a longer line of the body\nshort")
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "╭") || !strings.HasPrefix(lines[len(lines)-1], "╰") {
		t.Fatalf("no border:\n%s", b.String())
	}
	for _, l := range lines {
		if utf8.RuneCountInString(l) != utf8.RuneCountInString(lines[0]) {
			t.Fatalf("uneven:\n%s", b.String())
		}
	}
}

func TestPickKey(t *testing.T) {
	cases := []struct {
		i      int
		key    string
		next   int
		done   bool
		cancel bool
	}{
		{0, "\x1b[B", 1, false, false},
		{2, "\x1b[B", 0, false, false}, // wraps
		{0, "\x1b[A", 2, false, false}, // wraps
		{1, "\x1bOA", 0, false, false},
		{1, "\r", 1, true, false},
		{1, "\n", 1, true, false},
		{1, "\x03", 1, false, true},
		{1, "\x1b", 1, false, true},
		{0, "3", 2, false, false},
		{0, "4", 0, false, false}, // no such option
		{0, "x", 0, false, false},
	}
	for _, c := range cases {
		next, done, cancel := pickKey(c.i, 3, []byte(c.key))
		if next != c.next || done != c.done || cancel != c.cancel {
			t.Errorf("%d %q: got %d %v %v", c.i, c.key, next, done, cancel)
		}
	}
}
