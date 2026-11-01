// Command docref writes the API reference of the guide's plugin pages from
// the plugins themselves: their Go source and plugin.json. A page includes
// what it writes, so the reference can't drift from the code.
//
//	go run ./tools/docref -out ../nimbus-starter/resources/views/native/ref motion
//
// Go signatures don't say what a parameter is for, so a function's comment
// can, in lines godoc leaves out:
//
//	//nimbus:param hz readings a second; 30 if it's 0 or less
//	//nimbus:returns why it stopped
//	//nimbus:platforms ios
//
// With no names it does every plugin. It fails on a parameter with no
// description, so that nothing goes out half-documented.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("plugins", "plugins", "the plugins folder")
	out := flag.String("out", "", "where the view files go")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "docref: -out is required")
		os.Exit(2)
	}
	names := flag.Args()
	if len(names) == 0 {
		dirs, err := os.ReadDir(*root)
		if err != nil {
			fatal(err)
		}
		for _, d := range dirs {
			if d.IsDir() {
				names = append(names, d.Name())
			}
		}
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatal(err)
	}
	failed := false
	for _, name := range names {
		p, err := load(filepath.Join(*root, name), name)
		if err != nil {
			fatal(err)
		}
		for _, w := range p.missing {
			fmt.Fprintf(os.Stderr, "docref: %s: %s\n", name, w)
			failed = true
		}
		head, body := p.render()
		if err := os.WriteFile(filepath.Join(*out, name+"-head.nimbus"), []byte(head), 0o644); err != nil {
			fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*out, name+".nimbus"), []byte(body), 0o644); err != nil {
			fatal(err)
		}
	}
	if failed {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "docref:", err)
	os.Exit(1)
}
