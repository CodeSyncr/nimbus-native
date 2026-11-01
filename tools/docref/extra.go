package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// example is a plugin's complete program, as package main.
func example(dir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, name+".go"))
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%s has no example in %s", name, dir)
	}
	if err != nil {
		return "", err
	}
	src := strings.Replace(strings.TrimSpace(string(b)), "package guide", "package main", 1)
	return fmt.Sprintf(`<h2 id="example">A complete example</h2>`+"\n"+
		`<pre data-file="%s.go"><code class="language-go">%s</code></pre>`+"\n", name, esc(src)), nil
}

// result is one line of a plugin's "Tested on" table.
type result struct {
	Where  string `json:"where"`
	Result string `json:"result"`
}

func loadTested(path string) (map[string][]result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string][]result{}
	return out, json.Unmarshal(b, &out)
}

func renderTested(rows []result) string {
	var b strings.Builder
	b.WriteString(`<h2 id="tested">Tested on</h2>` + "\n")
	cells := make([][]string, len(rows))
	for i, r := range rows {
		cells[i] = []string{esc(r.Where), names(esc(r.Result))}
	}
	table(&b, []string{"Where", "Result"}, cells)
	return b.String()
}
