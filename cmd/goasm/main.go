package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	arch := flag.String("arch", "", "Architecture (arm64 or amd64)")
	in := flag.String("in", "", "Input template file")
	out := flag.String("out", "", "Output Go file")
	flag.Parse()

	if *arch == "" || *in == "" || *out == "" {
		flag.Usage()
		os.Exit(1)
	}

	inFile, err := os.Open(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer inFile.Close()

	parser := NewParser(inFile)
	templates, err := parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing templates: %v\n", err)
		os.Exit(1)
	}

	var encoder Encoder
	switch *arch {
	case "arm64":
		encoder = NewARM64Encoder(parser.aliases)
	case "amd64":
		encoder = NewAMD64Encoder(parser.aliases)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported architecture: %s\n", *arch)
		os.Exit(1)
	}

	outFile, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	generator := NewGenerator(*arch, encoder)
	if err := generator.Generate(outFile, templates); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s from %s\n", *out, *in)
}
