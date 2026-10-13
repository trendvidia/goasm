package gen

import (
	"fmt"
	"github.com/trendvidia/goasm/template"
	"regexp"
	"strconv"
	"strings"
)

// AMD64Encoder handles encoding of x86-64 instructions.
type AMD64Encoder struct {
	aliases map[string]string
}

func NewAMD64Encoder(aliases map[string]string) *AMD64Encoder {
	return &AMD64Encoder{aliases: aliases}
}

// operandError reports an operand the encoder cannot place. regOrParam
// panics with it from deep in an encoding, and Encode turns it back into an
// error, so an unrecognised operand fails generation instead of passing
// through into the output as if it were a register number.
type operandError struct{ op string }

func (e *AMD64Encoder) Encode(instr template.Instruction, params []template.Parameter) (code string, err error) {
	defer func() {
		if r := recover(); r != nil {
			oe, ok := r.(operandError)
			if !ok {
				panic(r)
			}
			code, err = "", fmt.Errorf("%s: unrecognised operand %q: not a register, alias or parameter", instr.Mnemonic, oe.op)
		}
	}()
	mnemonic := instr.Mnemonic
	ops := instr.Operands

	// Replace aliases
	for i, op := range ops {
		ops[i] = e.resolveAlias(op)
	}

	switch mnemonic {
	case "MOV":
		return e.encodeMov(ops, params)
	case "ADD":
		return e.encodeArithmetic("Add", ops, params)
	case "SUB":
		return e.encodeArithmetic("Sub", ops, params)
	case "MUL", "IMUL":
		if len(ops) == 2 && isImmOrParam(ops[1], params) {
			val := strings.TrimPrefix(ops[1], "#")
			return fmt.Sprintf("e.MulImm(%s, %s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[0], params), val), nil
		}
		return e.encodeArithmetic("Mul", ops, params)

	case "AND":
		if len(ops) == 2 && isImmOrParam(ops[1], params) {
			val := strings.TrimPrefix(ops[1], "#")
			return fmt.Sprintf("e.AndImm(%s, %s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[0], params), val), nil
		}
		return e.encodeArithmetic("And", ops, params)
	case "ORR", "OR":
		if len(ops) == 2 && isImmOrParam(ops[1], params) {
			val := strings.TrimPrefix(ops[1], "#")
			return fmt.Sprintf("e.OrrImm(%s, %s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[0], params), val), nil
		}
		return e.encodeArithmetic("Orr", ops, params)

	case "EOR", "XOR":
		if len(ops) == 2 && isImmOrParam(ops[1], params) {
			val := strings.TrimPrefix(ops[1], "#")
			return fmt.Sprintf("e.EorImm(%s, %s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[0], params), val), nil
		}
		return e.encodeArithmetic("Eor", ops, params)

	case "MOVZ": // ARM64 compatibility in templates? Usually not in AMD64
		return e.encodeMov(ops, params)
	case "MOVK":
		return e.encodeMov(ops, params)
	case "NEG":
		if len(ops) != 1 {
			return "", fmt.Errorf("invalid operands for NEG: %v", ops)
		}
		return fmt.Sprintf("e.NegReg(%s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[0], params)), nil
	case "MVN", "NOT":
		if len(ops) != 1 {
			return "", fmt.Errorf("invalid operands for NOT: %v", ops)
		}
		return fmt.Sprintf("e.MvnReg(%s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[0], params)), nil
	case "LSL", "SHL":
		return e.encodeShift("Shl", ops, params)
	case "LSR", "SHR":
		return e.encodeShift("Shr", ops, params)
	case "ASR", "SAR":
		return e.encodeShift("Sar", ops, params)
	case "CMP":
		if len(ops) != 2 {
			return "", fmt.Errorf("invalid operands for CMP: %v", ops)
		}
		if isImmOrParam(ops[1], params) {
			return fmt.Sprintf("e.CmpImm(%s, uint32(%s))", e.regOrParam(ops[0], params), strings.TrimPrefix(ops[1], "#")), nil
		}
		return fmt.Sprintf("e.CmpReg(%s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[1], params)), nil
	case "CSET":
		if len(ops) != 2 {
			return "", fmt.Errorf("invalid operands for CSET: %v", ops)
		}
		return fmt.Sprintf("e.Cset(%s, int(%s))", e.regOrParam(ops[0], params), e.condToCode(ops[1], params)), nil
	case "CSEL":
		if len(ops) != 4 {
			return "", fmt.Errorf("invalid operands for CSEL: %v", ops)
		}
		return fmt.Sprintf("e.Csel(%s, %s, %s, int(%s))", e.regOrParam(ops[0], params), e.regOrParam(ops[1], params), e.regOrParam(ops[2], params), e.condToCode(ops[3], params)), nil
	case "RET":
		return "e.Ret()", nil
	case "CALL":
		if len(ops) != 1 {
			return "", fmt.Errorf("invalid operands for CALL: %v", ops)
		}
		return fmt.Sprintf("e.Blr(%s)", e.regOrParam(ops[0], params)), nil
	case "B", "JMP":
		return e.encodeBranch(ops, params)
	case "B.EQ", "B.NE", "B.LT", "B.LE", "B.GT", "B.GE":
		if len(ops) != 1 {
			return "", fmt.Errorf("invalid operands for branch: %v", ops)
		}
		target := ops[0]
		retVar := ""
		if idx := strings.Index(target, "->"); idx != -1 {
			retVar = strings.TrimSpace(target[idx+2:])
			target = strings.TrimSpace(target[:idx])
		}
		condStr := strings.TrimPrefix(mnemonic, "B.")
		if target == "0" && retVar != "" {
			return fmt.Sprintf("%s = e.BcondPlaceholder(int(%s))", retVar, e.condToCode(condStr, params)), nil
		}
		return "", fmt.Errorf("%s %s: branches to labels are not supported; use the placeholder form `%s 0 -> name`", mnemonic, target, mnemonic)
	case "SBFX":

		if len(ops) != 4 {
			return "", fmt.Errorf("invalid operands for SBFX: %v", ops)
		}
		// SBFX rd, rn, lsb, width
		if ops[2] != "0" {
			return "", fmt.Errorf("SBFX: only lsb 0 is supported, got %s", ops[2])
		}
		return fmt.Sprintf("e.ExtractSigned(%s, %s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[1], params), ops[3]), nil
	case "UBFX":
		if len(ops) != 4 {
			return "", fmt.Errorf("invalid operands for UBFX: %v", ops)
		}
		if ops[2] != "0" {
			return "", fmt.Errorf("UBFX: only lsb 0 is supported, got %s", ops[2])
		}
		return fmt.Sprintf("e.ExtractUnsigned(%s, %s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[1], params), ops[3]), nil
	case "FADD":
		return e.encodeFloat("FaddD", ops, params)
	case "FSUB":
		return e.encodeFloat("FsubD", ops, params)
	case "FMUL":
		return e.encodeFloat("FmulD", ops, params)
	case "FDIV":
		return e.encodeFloat("FdivD", ops, params)
	case "FNEG":
		return e.encodeFloat("FnegD", ops, params)
	case "FCMP":
		if len(ops) != 2 {
			return "", fmt.Errorf("invalid operands for FCMP: %v", ops)
		}
		return fmt.Sprintf("e.FcmpD(%s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[1], params)), nil
	case "FMOV":
		if len(ops) != 2 {
			return "", fmt.Errorf("invalid operands for FMOV: %v", ops)
		}
		dst, src := ops[0], ops[1]
		if strings.HasPrefix(dst, "XMM") && !strings.HasPrefix(src, "XMM") {
			return fmt.Sprintf("e.FmovToFP(%s, %s)", e.regOrParam(dst, params), e.regOrParam(src, params)), nil
		} else if !strings.HasPrefix(dst, "XMM") && strings.HasPrefix(src, "XMM") {
			return fmt.Sprintf("e.FmovToGP(%s, %s)", e.regOrParam(dst, params), e.regOrParam(src, params)), nil
		}
		return fmt.Sprintf("e.fmovSD(%s, %s)", e.regOrParam(dst, params), e.regOrParam(src, params)), nil
	case "SCVTF":
		return fmt.Sprintf("e.IntToDouble(%s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[1], params)), nil
	case "FCVTZS":
		return fmt.Sprintf("e.DoubleToInt(%s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[1], params)), nil
	case "SaveCallRegs":
		return "e.SaveCallRegs()", nil
	case "RestoreCallRegs":
		return "e.RestoreCallRegs()", nil
	default:
		return "", fmt.Errorf("unsupported AMD64 mnemonic: %s", mnemonic)
	}
}

func (e *AMD64Encoder) resolveAlias(op string) string {
	if alias, ok := e.aliases[op]; ok {
		return alias
	}
	if strings.HasPrefix(op, "[") && strings.HasSuffix(op, "]") {
		inner := op[1 : len(op)-1]
		for k, v := range e.aliases {
			inner = regexp.MustCompile(`\b`+regexp.QuoteMeta(k)+`\b`).ReplaceAllLiteralString(inner, v)
		}
		return "[" + inner + "]"
	}
	return op
}

func (e *AMD64Encoder) encodeMov(ops []string, params []template.Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for MOV: %v", ops)
	}
	dst, src := ops[0], ops[1]

	if strings.HasPrefix(dst, "[") {
		mem := dst[1 : len(dst)-1]
		base, idx, scale, disp, err := e.parseMem(mem, params)
		_ = scale
		if err != nil {
			return "", err
		}
		if idx == "" {
			return fmt.Sprintf("e.StrImm(%s, %s, %s)", e.regOrParam(src, params), base, slotOffset(disp)), nil
		}
		return fmt.Sprintf("e.StrReg(%s, %s, %s)", e.regOrParam(src, params), base, idx), nil
	} else if strings.HasPrefix(src, "[") {
		mem := src[1 : len(src)-1]
		base, idx, scale, disp, err := e.parseMem(mem, params)
		_ = scale
		if err != nil {
			return "", err
		}
		if idx == "" {
			return fmt.Sprintf("e.LdrImm(%s, %s, %s)", e.regOrParam(dst, params), base, slotOffset(disp)), nil
		}
		return fmt.Sprintf("e.LdrReg(%s, %s, %s)", e.regOrParam(dst, params), base, idx), nil
	}

	if isImmOrParam(src, params) {
		val := strings.TrimPrefix(src, "#")
		return fmt.Sprintf("e.MovImm64(%s, uint64(%s))", e.regOrParam(dst, params), val), nil
	}

	return fmt.Sprintf("e.MovReg(%s, %s)", e.regOrParam(dst, params), e.regOrParam(src, params)), nil
}

func (e *AMD64Encoder) encodeArithmetic(method string, ops []string, params []template.Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for %s: %v", method, ops)
	}
	dst, src := ops[0], ops[1]
	if isImmOrParam(src, params) {
		val := strings.TrimPrefix(src, "#")
		return fmt.Sprintf("e.%sImm(%s, %s, uint64(%s))", method, e.regOrParam(dst, params), e.regOrParam(dst, params), val), nil
	}

	if strings.HasPrefix(src, "[") {
		return "", fmt.Errorf("%s with a memory source is not supported; load it into a register first", method)
	}
	return fmt.Sprintf("e.%sReg(%s, %s, %s)", method, e.regOrParam(dst, params), e.regOrParam(dst, params), e.regOrParam(src, params)), nil
}

func (e *AMD64Encoder) encodeShift(method string, ops []string, params []template.Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for shift: %v", ops)
	}
	dst, src := ops[0], ops[1]
	if isImmOrParam(src, params) {
		val := strings.TrimPrefix(src, "#")
		return fmt.Sprintf("e.%sImm(%s, %s, int(%s))", method, e.regOrParam(dst, params), e.regOrParam(dst, params), val), nil
	}
	// X86 variable shift MUST use RCX (CL).
	return fmt.Sprintf("e.%sReg(%s, %s, %s)", method, e.regOrParam(dst, params), e.regOrParam(dst, params), e.regOrParam(src, params)), nil
}

func (e *AMD64Encoder) encodeFloat(method string, ops []string, params []template.Parameter) (string, error) {
	if method == "FnegD" {
		if len(ops) != 2 {
			return "", fmt.Errorf("FnegD requires 2 operands")
		}
		return fmt.Sprintf("e.FnegD(%s, %s)", e.regOrParam(ops[0], params), e.regOrParam(ops[1], params)), nil
	}
	if len(ops) == 2 {
		return fmt.Sprintf("e.%s(%s, %s, %s)", method, e.regOrParam(ops[0], params), e.regOrParam(ops[0], params), e.regOrParam(ops[1], params)), nil
	}
	if len(ops) == 3 {
		return fmt.Sprintf("e.%s(%s, %s, %s)", method, e.regOrParam(ops[0], params), e.regOrParam(ops[1], params), e.regOrParam(ops[2], params)), nil
	}
	return "", fmt.Errorf("invalid operands for float %s: %v", method, ops)
}

func (e *AMD64Encoder) encodeBranch(ops []string, params []template.Parameter) (string, error) {
	if len(ops) != 1 {
		return "", fmt.Errorf("invalid operands for branch: %v", ops)
	}
	target := ops[0]
	retVar := ""
	if idx := strings.Index(target, "->"); idx != -1 {
		retVar = strings.TrimSpace(target[idx+2:])
		target = strings.TrimSpace(target[:idx])
	}
	if target == "0" && retVar != "" {
		return fmt.Sprintf("%s = e.BPlaceholder()", retVar), nil
	}
	return "", fmt.Errorf("B %s: branches to labels are not supported; use the placeholder form `B 0 -> name`", target)
}

func (e *AMD64Encoder) regOrParam(op string, params []template.Parameter) string {
	if isParam(op, params) {
		pType := ""
		for _, p := range params {
			if p.Name == op {
				pType = p.Type
				break
			}
		}
		if pType == "int" {
			return op
		}
		return fmt.Sprintf("int(%s)", op)
	}
	n := regToNumAMD64(op)
	if n == op {
		panic(operandError{op})
	}
	return n
}

func regToNumAMD64(reg string) string {
	regs := map[string]string{
		"RAX": "0", "RCX": "1", "RDX": "2", "RBX": "3", "RSP": "4", "RBP": "5", "RSI": "6", "RDI": "7",
		"R8": "8", "R9": "9", "R10": "10", "R11": "11", "R12": "12", "R13": "13", "R14": "14", "R15": "15",
		"XMM0": "0", "XMM1": "1", "XMM2": "2", "XMM3": "3", "XMM4": "4", "XMM5": "5", "XMM6": "6", "XMM7": "7",
	}
	if n, ok := regs[reg]; ok {
		return n
	}
	return reg
}

func (e *AMD64Encoder) condToCode(cond string, params []template.Parameter) string {
	if isParam(cond, params) {
		return cond
	}
	condMap := map[string]string{
		"EQ": "CondEQ", "NE": "CondNE", "LT": "CondLT", "LE": "CondLE", "GT": "CondGT", "GE": "CondGE",
		"LO": "CondLO", "LS": "CondLS", "HI": "CondHI", "HS": "CondHS",
	}

	if code, ok := condMap[cond]; ok {
		return code
	}
	return cond
}

// parseMem reads "base + index*8 + disp" (terms in any order, disp may be
// subtracted). The Emitter's indexed forms always scale by 8, and its
// displacement is a count of 8-byte slots, so anything else is an error.
func (e *AMD64Encoder) parseMem(mem string, params []template.Parameter) (base string, idx string, scale int, disp int, err error) {
	for _, t := range memTerms(mem) {
		p, neg := t.text, t.neg
		switch {
		case strings.Contains(p, "*"):
			sp := strings.Split(p, "*")
			scale, err = strconv.Atoi(strings.TrimSpace(sp[1]))
			if neg || err != nil || scale != 8 {
				return "", "", 0, 0, fmt.Errorf("memory operand [%s]: index must be added and scaled by 8, got %q", mem, p)
			}
			idx = e.regOrParam(strings.TrimSpace(sp[0]), params)
		case isRegister(p):
			if neg || base != "" {
				return "", "", 0, 0, fmt.Errorf("memory operand [%s]: one added base register is supported", mem)
			}
			base = e.regOrParam(p, params)
		default:
			v, convErr := strconv.Atoi(p)
			if convErr != nil {
				return "", "", 0, 0, fmt.Errorf("memory operand [%s]: unsupported part %q", mem, p)
			}
			if v%8 != 0 {
				return "", "", 0, 0, fmt.Errorf("memory operand [%s]: displacement %d is not a multiple of 8", mem, v)
			}
			if neg {
				v = -v
			}
			disp += v
		}
	}
	if base == "" {
		return "", "", 0, 0, fmt.Errorf("memory operand [%s]: no base register", mem)
	}
	return base, idx, scale, disp, nil
}

type memTerm struct {
	text string
	neg  bool
}

// memTerms splits "a + b - c" into signed terms.
func memTerms(mem string) []memTerm {
	var terms []memTerm
	neg := false
	start := 0
	flush := func(end int) {
		if t := strings.TrimSpace(mem[start:end]); t != "" {
			terms = append(terms, memTerm{t, neg})
		}
	}
	for i, c := range mem {
		if c == '+' || c == '-' {
			flush(i)
			neg = c == '-'
			start = i + 1
		}
	}
	flush(len(mem))
	return terms
}

// slotOffset renders a byte displacement as the Emitter's slot count: a
// signed 32-bit value carried in a uint32, so -8 bytes is 0xFFFFFFFF.
func slotOffset(disp int) string {
	if disp >= 0 {
		return strconv.Itoa(disp / 8)
	}
	return fmt.Sprintf("0x%X", uint32(int32(disp/8)))
}

func isRegister(r string) bool {
	regs := map[string]bool{
		"RAX": true, "RCX": true, "RDX": true, "RBX": true, "RSP": true, "RBP": true, "RSI": true, "RDI": true,
		"R8": true, "R9": true, "R10": true, "R11": true, "R12": true, "R13": true, "R14": true, "R15": true,
		"XMM0": true, "XMM1": true, "XMM2": true, "XMM3": true, "XMM4": true, "XMM5": true, "XMM6": true, "XMM7": true,
	}
	return regs[r]
}

// isImmOrParam reports whether op is an immediate: a parameter, a
// #-prefixed value, or a bare integer literal. Intel syntax writes
// immediates bare and always names registers, so `ADD SP, 8` adds 8.
func isImmOrParam(op string, params []template.Parameter) bool {
	if strings.HasPrefix(op, "#") || isParam(op, params) {
		return true
	}
	_, err := strconv.ParseInt(op, 0, 64)
	return err == nil
}
