package gen

import (
	"github.com/trendvidia/goasm/template"
	"strings"
	"testing"
)

func TestARM64Encoder(t *testing.T) {
	aliases := map[string]string{"BP": "X22"}
	enc := NewARM64Encoder(aliases)
	params := []template.Parameter{{Name: "src", Type: "uint8"}}

	tests := []struct {
		instr    template.Instruction
		expected string
	}{
		{
			template.Instruction{Mnemonic: "LDR", Operands: []string{"X24", "[BP, src, LSL #3]"}},
			"e.LdrReg(24, 22, int(src))",
		},

		{
			template.Instruction{Mnemonic: "ADD", Operands: []string{"X24", "X24", "X25"}},
			"e.AddReg(24, 24, 25)",
		},
	}

	for _, tc := range tests {
		got, err := enc.Encode(tc.instr, params)
		if err != nil {
			t.Errorf("Encode failed for %v: %v", tc.instr, err)
			continue
		}
		if !strings.Contains(got, tc.expected) {
			t.Errorf("For %v, expected to contain %s, got %s", tc.instr, tc.expected, got)
		}
	}
}

func TestAMD64Encoder(t *testing.T) {
	aliases := map[string]string{"BP": "R14"}
	enc := NewAMD64Encoder(aliases)
	params := []template.Parameter{{Name: "slot", Type: "uint8"}}

	tests := []struct {
		instr    template.Instruction
		expected string
	}{
		{
			// The emitter API since 6e4a188: a scaled load is one LdrReg
			// call, not raw REX bytes. The golden test over
			// templates/amd64.goasm holds the full output.
			template.Instruction{Mnemonic: "MOV", Operands: []string{"RAX", "[BP + slot*8]"}},
			"e.LdrReg(0, 14, int(slot))",
		},
	}

	for _, tc := range tests {
		got, err := enc.Encode(tc.instr, params)
		if err != nil {
			t.Errorf("Encode failed for %v: %v", tc.instr, err)
			continue
		}
		if !strings.Contains(got, tc.expected) {
			t.Errorf("For %v, expected to contain %s, got %s", tc.instr, tc.expected, got)
		}
	}
}
