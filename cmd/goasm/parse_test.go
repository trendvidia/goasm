package main

import (
	"strings"
	"testing"
)

func TestParser(t *testing.T) {
	input := `# ALIAS BP = X22
# TEMPLATE Test(dst uint8, src uint8)
    LDR X24, [BP, src]
    STR X24, [BP, dst]
`
	parser := NewParser(strings.NewReader(input))
	templates, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}

	tmpl := templates[0]
	if tmpl.Name != "Test" {
		t.Errorf("Expected template name Test, got %s", tmpl.Name)
	}

	if len(tmpl.Params) != 2 {
		t.Fatalf("Expected 2 params, got %d", len(tmpl.Params))
	}

	if tmpl.Params[0].Name != "dst" || tmpl.Params[0].Type != "uint8" {
		t.Errorf("Unexpected param 0: %v", tmpl.Params[0])
	}

	if len(tmpl.Instrs) != 2 {
		t.Fatalf("Expected 2 instructions, got %d", len(tmpl.Instrs))
	}

	if tmpl.Instrs[0].Mnemonic != "LDR" {
		t.Errorf("Expected mnemonic LDR, got %s", tmpl.Instrs[0].Mnemonic)
	}

	if parser.aliases["BP"] != "X22" {
		t.Errorf("Expected alias BP=X22, got %s", parser.aliases["BP"])
	}
}

func TestParseOperands(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"X24, [BP, src]", []string{"X24", "[BP, src]"}},
		{"X24, [BP, src, LSL #3]", []string{"X24", "[BP, src, LSL #3]"}},
		{"RAX, [RBP + slot*8]", []string{"RAX", "[RBP + slot*8]"}},
	}

	for _, tc := range tests {
		got := parseOperands(tc.input)
		if len(got) != len(tc.expected) {
			t.Errorf("For %s, expected %d operands, got %d", tc.input, len(tc.expected), len(got))
			continue
		}
		for i := range got {
			if got[i] != tc.expected[i] {
				t.Errorf("For %s, operand %d: expected %s, got %s", tc.input, i, tc.expected[i], got[i])
			}
		}
	}
}
