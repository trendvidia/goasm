package main

import (
	"fmt"
	"strconv"
	"strings"
)

// ARM64Encoder handles encoding of ARM64 instructions.
type ARM64Encoder struct {
	aliases map[string]string
}

func NewARM64Encoder(aliases map[string]string) *ARM64Encoder {
	return &ARM64Encoder{aliases: aliases}
}

func (e *ARM64Encoder) Encode(instr Instruction, params []Parameter) (string, error) {
	mnemonic := instr.Mnemonic
	ops := instr.Operands

	// Replace aliases in operands
	for i, op := range ops {
		ops[i] = e.resolveAlias(op)
	}

	switch mnemonic {
	case "LDR", "STR":
		return e.encodeLoadStore(mnemonic, ops, params)
	case "ADD", "SUB", "ORR", "EOR", "ORN":
		return e.encodeArithmetic(mnemonic, ops, params)
	case "AND":
		if len(ops) == 3 && strings.HasPrefix(ops[2], "#") {
			return e.encodeLogicalImm(mnemonic, ops, params)
		}
		return e.encodeArithmetic(mnemonic, ops, params)
	case "And48":
		if len(ops) != 2 {
			return "", fmt.Errorf("invalid operands for And48: %v", ops)
		}
		rd, rn := ops[0], ops[1]
		return fmt.Sprintf("e.And48(%s, %s)", e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0)), nil
	case "MOV":
		if len(ops) != 2 {
			return "", fmt.Errorf("invalid operands for MOV: %v", ops)
		}
		if strings.HasPrefix(ops[1], "#") || isParam(ops[1], params) {
			val := strings.TrimPrefix(ops[1], "#")
			return fmt.Sprintf("e.MovImm64(%s, uint64(%s))", e.regOrParam(ops[0], params, 0), val), nil
		}
		// Register-to-register MOV Rd, Rn is alias for ORR Rd, XZR, Rn
		return e.encodeArithmetic("ORR", []string{ops[0], "XZR", ops[1]}, params)

	case "MOVZ":
		return e.encodeMovz(ops, params)
	case "MOVK":
		return e.encodeMovk(ops, params)
	case "MUL":
		return e.encodeMultiply(ops, params)
	case "MVN":
		return e.encodeMvn(ops, params)
	case "LSLV", "LSRV", "ASRV":
		return e.encodeShiftVar(mnemonic, ops, params)
	case "CMP":
		return e.encodeCompare(ops, params)
	case "CSET":
		return e.encodeCset(ops, params)
	case "CSEL":
		return e.encodeCsel(ops, params)
	case "B", "B.EQ", "B.NE", "B.LT", "B.LE", "B.GT", "B.GE":
		return e.encodeBranch(mnemonic, ops, params)
	case "SBFX", "UBFX":
		return e.encodeBitfield(mnemonic, ops, params)
	case "FADD", "FSUB", "FMUL", "FDIV":
		return e.encodeFloatArithmetic(mnemonic, ops, params)
	case "FNEG":
		return e.encodeFloatUnary(mnemonic, ops, params)
	case "FCMP":
		return e.encodeFloatCompare(ops, params)
	case "FMOV":
		return e.encodeFloatMove(ops, params)
	case "SCVTF", "FCVTZS":
		return e.encodeFloatConvert(mnemonic, ops, params)
	case "BLR":
		if len(ops) != 1 {
			return "", fmt.Errorf("invalid operands for BLR: %v", ops)
		}
		return fmt.Sprintf("e.Blr(%s)", e.regOrParam(ops[0], params, 0)), nil
	case "BL":
		if len(ops) != 1 {
			return "", fmt.Errorf("invalid operands for BL: %v", ops)
		}
		return fmt.Sprintf("e.Bl(%s)", e.regOrParam(ops[0], params, 0)), nil
	case "SaveCallRegs":
		return "e.SaveCallRegs()", nil
	case "RestoreCallRegs":
		return "e.RestoreCallRegs()", nil
	case "RET":
		return "e.Ret()", nil
	default:
		return "", fmt.Errorf("unsupported ARM64 mnemonic: %s", mnemonic)
	}
}

func (e *ARM64Encoder) encodeBitfield(mnemonic string, ops []string, params []Parameter) (string, error) {
	if len(ops) != 4 {
		return "", fmt.Errorf("invalid operands for %s: %v", mnemonic, ops)
	}
	rd, rn, lsb, width := ops[0], ops[1], ops[2], ops[3]
	if mnemonic == "SBFX" {
		return fmt.Sprintf("e.Sbfx(%s, %s, %s, %s)", 
			e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0), lsb, width), nil
	}
	if lsb == "0" {
		return fmt.Sprintf("e.ExtractUnsigned(%s, %s, %s)", 
			e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0), width), nil
	}
	return "", fmt.Errorf("unsupported bitfield: %s %v", mnemonic, ops)
}

func (e *ARM64Encoder) encodeFloatArithmetic(mnemonic string, ops []string, params []Parameter) (string, error) {
	if len(ops) != 3 {
		return "", fmt.Errorf("invalid operands for %s: %v", mnemonic, ops)
	}
	fd, fn, fm := ops[0], ops[1], ops[2]
	method := ""
	switch mnemonic {
	case "FADD": method = "FaddD"
	case "FSUB": method = "FsubD"
	case "FMUL": method = "FmulD"
	case "FDIV": method = "FdivD"
	}
	return fmt.Sprintf("e.%s(%s, %s, %s)", method, e.regOrParam(fd, params, 0), e.regOrParam(fn, params, 0), e.regOrParam(fm, params, 0)), nil
}

func (e *ARM64Encoder) encodeFloatUnary(mnemonic string, ops []string, params []Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for %s: %v", mnemonic, ops)
	}
	fd, fn := ops[0], ops[1]
	return fmt.Sprintf("e.FnegD(%s, %s)", e.regOrParam(fd, params, 0), e.regOrParam(fn, params, 0)), nil
}

func (e *ARM64Encoder) encodeFloatCompare(ops []string, params []Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for FCMP: %v", ops)
	}
	fn, fm := ops[0], ops[1]
	return fmt.Sprintf("e.FcmpD(%s, %s)", e.regOrParam(fn, params, 0), e.regOrParam(fm, params, 0)), nil
}

func (e *ARM64Encoder) encodeFloatMove(ops []string, params []Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for FMOV: %v", ops)
	}
	dst, src := ops[0], ops[1]
	if (strings.HasPrefix(dst, "D") || isParam(dst, params)) && !strings.HasPrefix(src, "D") {
		return fmt.Sprintf("e.FmovToFP(%s, %s)", e.regOrParam(dst, params, 0), e.regOrParam(src, params, 0)), nil
	} else if !strings.HasPrefix(dst, "D") && (strings.HasPrefix(src, "D") || isParam(src, params)) {
		return fmt.Sprintf("e.FmovToGP(%s, %s)", e.regOrParam(dst, params, 0), e.regOrParam(src, params, 0)), nil
	}
	return "", fmt.Errorf("unsupported FMOV form: %s, %s", dst, src)
}

func (e *ARM64Encoder) encodeFloatConvert(mnemonic string, ops []string, params []Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for %s: %v", mnemonic, ops)
	}
	dst, src := ops[0], ops[1]
	if mnemonic == "SCVTF" {
		return fmt.Sprintf("e.IntToDouble(%s, %s)", e.regOrParam(dst, params, 0), e.regOrParam(src, params, 0)), nil
	} else {
		return fmt.Sprintf("e.DoubleToInt(%s, %s)", e.regOrParam(dst, params, 0), e.regOrParam(src, params, 0)), nil
	}
}

func (e *ARM64Encoder) resolveAlias(op string) string {
	if alias, ok := e.aliases[op]; ok {
		return alias
	}
	if strings.HasPrefix(op, "[") && strings.HasSuffix(op, "]") {
		inner := op[1 : len(op)-1]
		parts := strings.Split(inner, ",")
		for i, p := range parts {
			p = strings.TrimSpace(p)
			if alias, ok := e.aliases[p]; ok {
				parts[i] = alias
			}
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return op
}

func (e *ARM64Encoder) regOrParam(op string, params []Parameter, shift int) string {
	if isParam(op, params) {
		pType := ""
		for _, p := range params {
			if p.Name == op {
				pType = p.Type
				break
			}
		}
		if pType == "int" {
			if shift == 0 {
				return op
			}
			return fmt.Sprintf("%s<<%d", op, shift)
		}
		if shift == 0 {
			return fmt.Sprintf("int(%s)", op)
		}
		return fmt.Sprintf("int(%s)<<%d", op, shift)
	}
	num := regToNum(op)
	if shift == 0 {
		return num
	}
	if n, err := strconv.ParseUint(num, 0, 32); err == nil {
		return fmt.Sprintf("0x%X", n<<shift)
	}
	return fmt.Sprintf("int(%s)<<%d", num, shift)
}


func (e *ARM64Encoder) encodeLoadStore(mnemonic string, ops []string, params []Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for %s: %v", mnemonic, ops)
	}
	rt := ops[0]
	mem := ops[1]
	if !strings.HasPrefix(mem, "[") || !strings.HasSuffix(mem, "]") {
		return "", fmt.Errorf("invalid memory operand for %s: %s", mnemonic, mem)
	}
	inner := mem[1 : len(mem)-1]
	parts := strings.Split(inner, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}


	rn := parts[0]
	
	if len(parts) > 1 {
		// Check for register offset: [Rn, Rm, LSL #shift] or [Rn, Rm]
		// Actually, let's look for LSL or if the second part is a register/param.
		isRegOffset := false
		var rm string
		for i := 1; i < len(parts); i++ {
			p := parts[i]
			if strings.Contains(p, "LSL") {
				isRegOffset = true
				parts2 := strings.Split(p, "LSL")
				rmCandidate := strings.TrimSpace(parts2[0])
				if rmCandidate != "" {
					rm = rmCandidate
				} else if i > 1 {
					// src, LSL #3 case
					rm = parts[i-1]
				}
				break
			}
		}

		if isRegOffset || (!strings.HasPrefix(parts[1], "#") && !isParam(parts[1], params) && regToNum(parts[1]) != parts[1]) {
			// Actually simpler: if it's not starting with # and it's not just a numeric param, it's likely a reg offset
			// But for now, specifically handle the common forms.
			if rm == "" { rm = parts[1] }
			rm = strings.TrimPrefix(rm, "#")

			method := "LdrReg"
			if mnemonic == "STR" { method = "StrReg" }
			return fmt.Sprintf("e.%s(%s, %s, %s)", method, e.regOrParam(rt, params, 0), e.regOrParam(rn, params, 0), e.regOrParam(rm, params, 0)), nil
		} else {
			imm := strings.TrimPrefix(parts[1], "#")
			method := "LdrImm"
			if mnemonic == "STR" { method = "StrImm" }
			return fmt.Sprintf("e.%s(%s, %s, uint32(%s))", method, e.regOrParam(rt, params, 0), e.regOrParam(rn, params, 0), e.regOrParam(imm, params, 0)), nil
		}
	}


	method := "LdrImm"
	if mnemonic == "STR" { method = "StrImm" }
	return fmt.Sprintf("e.%s(%s, %s, 0)", method, e.regOrParam(rt, params, 0), e.regOrParam(rn, params, 0)), nil
}


func (e *ARM64Encoder) encodeMovz(ops []string, params []Parameter) (string, error) {
	if len(ops) < 2 {
		return "", fmt.Errorf("invalid operands for MOVZ: %v", ops)
	}
	rd, imm := ops[0], ops[1]
	shift := 0
	if len(ops) > 2 && strings.HasPrefix(ops[2], "LSL") {
		s := strings.TrimPrefix(ops[2], "LSL #")
		shift, _ = strconv.Atoi(s)
	}
	return fmt.Sprintf("e.Movz(%s, uint16(%s), %d)", e.regOrParam(rd, params, 0), strings.TrimPrefix(imm, "#"), shift), nil
}

func (e *ARM64Encoder) encodeMovk(ops []string, params []Parameter) (string, error) {
	if len(ops) < 2 {
		return "", fmt.Errorf("invalid operands for MOVK: %v", ops)
	}
	rd, imm := ops[0], ops[1]
	shift := 0
	if len(ops) > 2 && strings.HasPrefix(ops[2], "LSL") {
		s := strings.TrimPrefix(ops[2], "LSL #")
		shift, _ = strconv.Atoi(s)
	}
	return fmt.Sprintf("e.Movk(%s, uint16(%s), %d)", e.regOrParam(rd, params, 0), strings.TrimPrefix(imm, "#"), shift), nil
}

func (e *ARM64Encoder) encodeLogicalImm(mnemonic string, ops []string, params []Parameter) (string, error) {
	if len(ops) != 3 {
		return "", fmt.Errorf("invalid operands for %s immediate: %v", mnemonic, ops)
	}
	rd, rn, imm := ops[0], ops[1], ops[2]
	if imm == "#mask48" {
		// AND rd, rn, #mask48 -> e.emit32(0x92000000 | 1<<22 | 16<<16 | 47<<10 | uint32(rn)<<5 | uint32(rd))
		return fmt.Sprintf("e.And48(%s, %s)", e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0)), nil
	}


	return "", fmt.Errorf("unsupported logical immediate: %s", imm)
}

func (e *ARM64Encoder) encodeArithmetic(mnemonic string, ops []string, params []Parameter) (string, error) {
	if len(ops) != 3 {
		return "", fmt.Errorf("invalid operands for %s: %v", mnemonic, ops)
	}
	rd, rn, rm := ops[0], ops[1], ops[2]
	isImm := strings.HasPrefix(rm, "#")
	immVal := strings.TrimPrefix(rm, "#")
	method := ""
	switch mnemonic {
	case "ADD": method = "Add"
	case "SUB": method = "Sub"
	case "AND": method = "And"
	case "ORR": method = "Orr"
	case "EOR": method = "Eor"
	case "ORN": method = "Orn"
	}
	if isImm {
		return fmt.Sprintf("e.%sImm(%s, %s, uint32(%s))", method, e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0), e.regOrParam(immVal, params, 0)), nil
	}
	return fmt.Sprintf("e.%sReg(%s, %s, %s)", method, e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0), e.regOrParam(rm, params, 0)), nil
}

func (e *ARM64Encoder) encodeMultiply(ops []string, params []Parameter) (string, error) {
	if len(ops) != 3 {
		return "", fmt.Errorf("invalid operands for MUL: %v", ops)
	}
	rd, rn, rm := ops[0], ops[1], ops[2]
	return fmt.Sprintf("e.MulReg(%s, %s, %s)", e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0), e.regOrParam(rm, params, 0)), nil
}

func (e *ARM64Encoder) encodeMvn(ops []string, params []Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for MVN: %v", ops)
	}
	rd, rn := ops[0], ops[1]
	return fmt.Sprintf("e.MvnReg(%s, %s)", e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0)), nil
}

func (e *ARM64Encoder) encodeShiftVar(mnemonic string, ops []string, params []Parameter) (string, error) {
	if len(ops) != 3 {
		return "", fmt.Errorf("invalid operands for %s: %v", mnemonic, ops)
	}
	rd, rn, rm := ops[0], ops[1], ops[2]
	method := ""
	switch mnemonic {
	case "LSLV": method = "LslvReg"
	case "LSRV": method = "LsrvReg"
	case "ASRV": method = "AsrvReg"
	}
	return fmt.Sprintf("e.%s(%s, %s, %s)", method, e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0), e.regOrParam(rm, params, 0)), nil
}

func (e *ARM64Encoder) encodeCompare(ops []string, params []Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for CMP: %v", ops)
	}
	rn, rm := ops[0], ops[1]
	return fmt.Sprintf("e.CmpReg(%s, %s)", e.regOrParam(rn, params, 0), e.regOrParam(rm, params, 0)), nil
}

func (e *ARM64Encoder) encodeCset(ops []string, params []Parameter) (string, error) {
	if len(ops) != 2 {
		return "", fmt.Errorf("invalid operands for CSET: %v", ops)
	}
	rd, cond := ops[0], ops[1]
	condStr := cond
	if !isParam(cond, params) {
		condStr = fmt.Sprintf("condToCode(\"%s\")", cond)
	} else {
		condStr = fmt.Sprintf("condToCode(%s)", cond)
	}
	return fmt.Sprintf("e.Cset(%s, int(%s))", e.regOrParam(rd, params, 0), condStr), nil
}

func (e *ARM64Encoder) encodeCsel(ops []string, params []Parameter) (string, error) {
	if len(ops) != 4 {
		return "", fmt.Errorf("invalid operands for CSEL: %v", ops)
	}
	rd, rn, rm, cond := ops[0], ops[1], ops[2], ops[3]
	condStr := cond
	if !isParam(cond, params) {
		condStr = fmt.Sprintf("condToCode(\"%s\")", cond)
	} else {
		condStr = fmt.Sprintf("condToCode(%s)", cond)
	}
	return fmt.Sprintf("e.Csel(%s, %s, %s, int(%s))", e.regOrParam(rd, params, 0), e.regOrParam(rn, params, 0), e.regOrParam(rm, params, 0), condStr), nil
}

func (e *ARM64Encoder) condToCode(cond string, params []Parameter) string {
	if isParam(cond, params) {
		return cond
	}
	condMap := map[string]string{
		"EQ": "0", "NE": "1", "CS": "2", "HS": "2", "CC": "3", "LO": "3",
		"MI": "4", "PL": "5", "VS": "6", "VC": "7", "HI": "8", "LS": "9",
		"GE": "10", "LT": "11", "GT": "12", "LE": "13", "AL": "14", "NV": "15",
	}
	if code, ok := condMap[cond]; ok {
		return code
	}
	return cond
}

func (e *ARM64Encoder) encodeBranch(mnemonic string, ops []string, params []Parameter) (string, error) {
	if len(ops) != 1 {
		return "", fmt.Errorf("invalid operands for %s: %v", mnemonic, ops)
	}
	target := ops[0]
	retVar := ""
	if idx := strings.Index(target, "->"); idx != -1 {
		retVar = strings.TrimSpace(target[idx+2:])
		target = strings.TrimSpace(target[:idx])
	}
	if mnemonic == "B" {
		if target == "0" && retVar != "" {
			return fmt.Sprintf("%s = e.BPlaceholder()", retVar), nil
		}
		if isParam(target, params) {
			return fmt.Sprintf("e.B(e.Len(), int(%s))", target), nil
		}
		return fmt.Sprintf("// TODO: Branch to %s", target), nil
	}
	if strings.HasPrefix(mnemonic, "B.") {
		condStr := strings.TrimPrefix(mnemonic, "B.")
		if target == "0" && retVar != "" {
			return fmt.Sprintf("%s = e.BcondPlaceholder(int(condToCode(\"%s\")))", retVar, condStr), nil
		}
	}
	return "", fmt.Errorf("unsupported branch format: %s %v", mnemonic, ops)
}

func isParam(name string, params []Parameter) bool {
	for _, p := range params {
		if p.Name == name {
			return true
		}
	}
	return false
}

func regToNum(reg string) string {
	if strings.HasPrefix(reg, "X") || strings.HasPrefix(reg, "R") || strings.HasPrefix(reg, "D") {
		num := reg[1:]
		if n, err := strconv.Atoi(num); err == nil {
			return fmt.Sprintf("%d", n)
		}
	}
	switch reg {
	case "SP": return "31"
	case "XZR": return "31"
	}
	return reg
}
