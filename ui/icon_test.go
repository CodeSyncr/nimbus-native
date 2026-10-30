package ui

import (
	"strconv"
	"strings"
	"testing"
)

func TestIconPathsParse(t *testing.T) {
	takes := map[byte]int{'M': 2, 'L': 2, 'Q': 4, 'C': 6, 'O': 3, 'R': 5, 'Z': 1}
	if len(iconPaths) < 50 {
		t.Fatalf("only %d icons", len(iconPaths))
	}
	for name, path := range iconPaths {
		f := strings.Fields(path)
		for i := 0; i < len(f); {
			n, ok := takes[f[i][0]]
			if !ok || i+n > len(f) {
				t.Fatalf("%s: bad command at %d: %q", name, i, f[i])
			}
			nums := append([]string{f[i][1:]}, f[i+1:i+n]...)
			if f[i][0] != 'Z' {
				for _, v := range nums {
					if x, err := strconv.ParseFloat(v, 32); err != nil || x < 0 || x > 24 {
						t.Fatalf("%s: bad number %q in %q", name, v, f[i])
					}
				}
			}
			i += n
		}
	}
}
