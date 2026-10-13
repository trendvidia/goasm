// SPDX-License-Identifier: MIT
// Copyright (c) 2026 TrendVidia, LLC.

package gen

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/trendvidia/goasm/template"
)

var oneTemplate = []template.Template{{
	Name:   "Nop",
	Instrs: []template.Instruction{{Mnemonic: "RET"}},
}}

func TestGeneratePackage(t *testing.T) {
	for _, tc := range []struct{ pkg, want string }{
		{"", "package jit\n"},
		{"emit", "package emit\n"},
	} {
		g := NewGenerator("arm64", NewARM64Encoder(nil))
		g.Package = tc.pkg
		var out bytes.Buffer
		if err := g.Generate(&out, oneTemplate); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Errorf("Package %q: output lacks %q:\n%s", tc.pkg, tc.want, out.String())
		}
	}
}

type brokenEncoder struct{}

func (brokenEncoder) Encode(template.Instruction, []template.Parameter) (string, error) {
	return "e.Ret(", nil
}

// A formatting failure is reported in the error, with the unformatted
// source, and nothing is written to stdout or to w.
func TestGenerateFormatErrorStaysInError(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	var out bytes.Buffer
	genErr := NewGenerator("arm64", brokenEncoder{}).Generate(&out, oneTemplate)
	os.Stdout = stdout
	w.Close()
	var printed bytes.Buffer
	printed.ReadFrom(r)

	if genErr == nil {
		t.Fatal("Generate succeeded on malformed Go")
	}
	if !strings.Contains(genErr.Error(), "e.Ret(") {
		t.Errorf("error does not carry the unformatted source: %v", genErr)
	}
	if printed.Len() != 0 {
		t.Errorf("Generate printed to stdout: %q", printed.String())
	}
	if out.Len() != 0 {
		t.Errorf("Generate wrote %d bytes to w on failure", out.Len())
	}
}
