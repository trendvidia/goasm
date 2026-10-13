// Command goasm compiles goasm templates into Go emitter methods.
//
//	go run github.com/trendvidia/goasm/cmd/goasm@<tag> -arch arm64 -in templates/arm64.goasm -out jit/emit_arm64.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/trendvidia/goasm/gen"
	"github.com/trendvidia/goasm/template"
)

func main() {
	arch := flag.String("arch", "", "Architecture (arm64 or amd64)")
	in := flag.String("in", "", "Input template file")
	out := flag.String("out", "", "Output Go file")
	pkg := flag.String("pkg", "jit", "Package name of the output file")
	flag.Parse()
	if *arch == "" || *in == "" || *out == "" {
		flag.Usage()
		os.Exit(1)
	}
	if err := run(*arch, *in, *out, *pkg); err != nil {
		fmt.Fprintln(os.Stderr, "goasm:", err)
		os.Exit(1)
	}
	fmt.Printf("Generated %s from %s\n", *out, *in)
}

func run(arch, in, out, pkg string) error {
	templates, aliases, err := template.ParseFile(in)
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}
	encoder, err := gen.NewEncoder(arch, aliases)
	if err != nil {
		return err
	}
	// Generate in memory first so a failed run leaves the previous output
	// in place rather than an empty file.
	var buf bytes.Buffer
	g := gen.NewGenerator(arch, encoder)
	g.Package = pkg
	if err := g.Generate(&buf, templates); err != nil {
		return err
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
}
