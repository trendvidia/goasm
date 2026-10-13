// SPDX-License-Identifier: MIT
// Copyright (c) 2026 TrendVidia, LLC.

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

// Each case was miscompiled before #1: an immediate read as a register, a
// branch to a label emitted as a comment, a subtracted displacement folded
// into the base register's number, a wide parameter truncated.
func TestEncodeOperandForms(t *testing.T) {
	amd := NewAMD64Encoder(map[string]string{"SP": "R12", "TAG": "R8"})
	arm := NewARM64Encoder(nil)
	wide := []template.Parameter{{Name: "v", Type: "uint64"}, {Name: "s", Type: "uint16"}}

	for _, tc := range []struct {
		enc     Encoder
		mnem    string
		ops     []string
		want    string // generated code; empty when an error is expected
		wantErr string
	}{
		{amd, "ADD", []string{"SP", "8"}, "e.AddImm(12, 12, uint64(8))", ""},
		{amd, "SUB", []string{"SP", "#8"}, "e.SubImm(12, 12, uint64(8))", ""},
		{amd, "SUB", []string{"SP", "0x10"}, "e.SubImm(12, 12, uint64(0x10))", ""},
		{amd, "SUB", []string{"RAX", "TAG"}, "e.SubReg(0, 0, 8)", ""},
		{amd, "AND", []string{"RAX", "255"}, "e.AndImm(0, 0, 255)", ""},
		{amd, "MOV", []string{"RAX", "[SP - 8]"}, "e.LdrImm(0, 12, 0xFFFFFFFF)", ""},
		{amd, "MOV", []string{"[SP - 16]", "RCX"}, "e.StrImm(1, 12, 0xFFFFFFFE)", ""},
		{amd, "MOV", []string{"RAX", "[SP + 16]"}, "e.LdrImm(0, 12, 2)", ""},
		{amd, "MOV", []string{"RAX", "[SP + RCX*8]"}, "e.LdrReg(0, 12, 1)", ""},
		{amd, "MOV", []string{"RSP", "RAX"}, "e.MovReg(4, 0)", ""},
		{amd, "MOV", []string{"RAX", "[SP + 4]"}, "", "not a multiple of 8"},
		{amd, "MOV", []string{"RAX", "[SP + RCX*4]"}, "", "scaled by 8"},
		{amd, "MOV", []string{"[SP]", "8"}, "", `unrecognised operand "8"`},
		{amd, "ADD", []string{"FOO", "RAX"}, "", `unrecognised operand "FOO"`},
		{amd, "ADD", []string{"RAX", "[SP]"}, "", "memory source"},
		{amd, "SBFX", []string{"RAX", "RAX", "8", "48"}, "", "only lsb 0"},
		{amd, "B", []string{".loop"}, "", "labels are not supported"},
		{amd, "B.NE", []string{".loop"}, "", "labels are not supported"},
		{arm, "B", []string{".loop"}, "", "labels are not supported"},
		{arm, "MOVZ", []string{"X25", "v", "LSL #0"}, "e.Movz(25, uint16(v), 0)", ""},
		{arm, "MOVK", []string{"X25", "v", "LSL #16"}, "e.Movk(25, uint16(v>>16), 16)", ""},
		{arm, "MOVK", []string{"X25", "v", "LSL #48"}, "e.Movk(25, uint16(v>>48), 48)", ""},
		{arm, "MOVK", []string{"X25", "s", "LSL #16"}, "e.Movk(25, uint16(s), 16)", ""},
		{arm, "MOVK", []string{"X25", "#0xABCD", "LSL #32"}, "e.Movk(25, uint16(0xABCD), 32)", ""},
		{arm, "MOVK", []string{"X25", "v", "LSL #12"}, "", "shift must be"},
	} {
		ops := append([]string(nil), tc.ops...)
		got, err := tc.enc.Encode(template.Instruction{Mnemonic: tc.mnem, Operands: ops}, wide)
		name := tc.mnem + " " + strings.Join(tc.ops, ", ")
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%T %s: got (%q, %v), want an error containing %q", tc.enc, name, got, err, tc.wantErr)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%T %s: got (%q, %v), want %q", tc.enc, name, got, err, tc.want)
		}
	}
}
