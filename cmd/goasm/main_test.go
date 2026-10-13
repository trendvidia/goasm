// SPDX-License-Identifier: MIT
// Copyright (c) 2026 TrendVidia, LLC.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A run that fails leaves the previous output file as it was.
func TestRunFailureKeepsOutput(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "bad.goasm")
	out := filepath.Join(dir, "emit_arm64.go")
	if err := os.WriteFile(in, []byte("# TEMPLATE X()\n    FROB X1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const previous = "previous output\n"
	if err := os.WriteFile(out, []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run("arm64", in, out, "jit"); err == nil {
		t.Fatal("run succeeded on an unknown mnemonic")
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != previous {
		t.Errorf("output after a failed run = %q, want %q", got, previous)
	}
}
