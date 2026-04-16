package jit

import "encoding/binary"

// Emitter is responsible for emitting machine code into a buffer.
type Emitter struct {
	Code []byte
}

// NewEmitter creates a new Emitter.
func NewEmitter() *Emitter {
	return &Emitter{
		Code: make([]byte, 0, 4096),
	}
}

func (e *Emitter) Len() int { return len(e.Code) }

// emit8 appends a single byte to the code buffer.
func (e *Emitter) emit8(b uint8) {
	e.Code = append(e.Code, b)
}

// emit16 appends a 16-bit little-endian value to the code buffer.
func (e *Emitter) emit16(v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	e.Code = append(e.Code, b[:]...)
}

// emit32 appends a 32-bit little-endian value to the code buffer.
func (e *Emitter) emit32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	e.Code = append(e.Code, b[:]...)
}

// emit64 appends a 64-bit little-endian value to the code buffer.
func (e *Emitter) emit64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	e.Code = append(e.Code, b[:]...)
}

// emitREX appends a REX prefix byte.
func (e *Emitter) emitREX(w, r, x, b uint8) {
	e.emit8(0x40 | (w << 3) | (r << 2) | (x << 1) | b)
}

// emitModRM appends a ModRM byte.
func (e *Emitter) emitModRM(mod, reg, rm uint8) {
	e.emit8((mod << 6) | (reg << 3) | rm)
}

// --- ARM64 register names (X0–X30, SP=31) ---
const (
	X0  = 0
	X1  = 1
	X2  = 2
	X3  = 3
	X19 = 19
	X20 = 20
	X21 = 21
	X22 = 22
	X23 = 23
	X24 = 24
	X25 = 25
	X26 = 26
	X27 = 27
	X28 = 28
	X29 = 29
	X30 = 30
	SP  = 31
	XZR = 31
)

func (e *Emitter) AddReg(rd, rn, rm int) {
	e.emit32(0x8B000000 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) AddImm(rd, rn int, imm uint32) {
	e.emit32(0x91000000 | (imm&0xFFF)<<10 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) AddImm64(rd, rn int, imm uint64) {
	e.MovImm64(X25, imm)
	e.AddReg(rd, rn, X25)
}

func (e *Emitter) SubImm(rd, rn int, imm uint32) {
	e.emit32(0xD1000000 | (imm&0xFFF)<<10 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) SubImm64(rd, rn int, imm uint64) {
	e.MovImm64(X25, imm)
	e.SubReg(rd, rn, X25)
}




func (e *Emitter) MulReg(rd, rn, rm int) {
	e.emit32(0x9B007C00 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}


func (e *Emitter) MulImm(rd, rn int, imm uint64) {
	e.MovImm64(X25, imm)
	e.MulReg(rd, rn, X25)
}


func (e *Emitter) MvnReg(rd, rn int) {
	e.emit32(0xAA200000 | uint32(rn)<<16 | 31<<5 | uint32(rd))
}

func (e *Emitter) MovReg(rd, rn int) {
	e.OrrReg(rd, XZR, rn)
}

func (e *Emitter) OrrReg(rd, rn, rm int) {
	e.emit32(0xAA000000 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) AddImm32(rd, rn int, imm uint32) { e.AddImm(rd, rn, imm) }
func (e *Emitter) SubImm32(rd, rn int, imm uint32) { e.SubImm(rd, rn, imm) }

func (e *Emitter) LslVar(rd, rn, rm int) { e.LslvReg(rd, rn, rm) }
func (e *Emitter) LsrVar(rd, rn, rm int) { e.LsrvReg(rd, rn, rm) }

func (e *Emitter) LslImm(rd, rn, imm int) { e.ShlImm(rd, rn, imm) }
func (e *Emitter) LsrImm(rd, rn, imm int) { e.ShrImm(rd, rn, imm) }

func (e *Emitter) OrrImm(rd, rn int, imm uint64) {
	// Simple implementation for common masks
	if imm == uint64(TAG_INT) {
		e.emit32(0xAA0003E0 | uint32(28)<<16 | uint32(rn)<<5 | uint32(rd)) // ORR rd, rn, X28
		return
	}
	e.emit32(0xB2000000 | uint32(rn)<<5 | uint32(rd))
}



func (e *Emitter) AndReg(rd, rn, rm int) {
	e.emit32(0x8A000000 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) AndImm(rd, rn int, imm uint64) {
	// Simple implementation for common masks
	if imm == 0x0000FFFFFFFFFFFF {
		e.And48(rd, rn)
		return
	}
	e.emit32(0x92000000 | uint32(rn)<<5 | uint32(rd)) // TODO: full bitmask immediate
}


func (e *Emitter) EorReg(rd, rn, rm int) {
	e.emit32(0xCA000000 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) EorImm(rd, rn int, imm uint64) {
	e.emit32(0xD2000000 | uint32(rn)<<5 | uint32(rd)) // TODO: full bitmask immediate
}


func (e *Emitter) LslvReg(rd, rn, rm int) {
	e.emit32(0x9AC02000 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) AsrvReg(rd, rn, rm int) {
	e.emit32(0x9AC02800 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) LsrvReg(rd, rn, rm int) {
	e.emit32(0x9AC02400 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) ShlImm(rd, rn, imm int) {
	r := (64 - imm) % 64
	s := 63 - imm
	e.emit32(0xD3400000 | 1<<22 | uint32(r)<<16 | uint32(s)<<10 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) ShrImm(rd, rn, imm int) {
	e.emit32(0xD3400000 | 1<<22 | uint32(imm)<<16 | 63<<10 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) SarImm(rd, rn, imm int) {
	e.emit32(0x93400000 | 1<<22 | uint32(imm)<<16 | 63<<10 | uint32(rn)<<5 | uint32(rd))
}


func (e *Emitter) CmpReg(rn, rm int) {
	e.emit32(0xEB000000 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(XZR))
}

func (e *Emitter) CmpImm(rn int, imm uint32) {
	e.emit32(0xF1000000 | (imm&0xFFF)<<10 | uint32(rn)<<5 | uint32(XZR))
}

func (e *Emitter) NegReg(rd, rn int) {
	e.SubReg(rd, XZR, rn)
}

func (e *Emitter) Cset(rd int, cond int) {
	inv := e.invertCond(uint32(cond))
	e.emit32(0x9A9F0400 | uint32(inv)<<12 | uint32(XZR)<<5 | uint32(rd))
}


func (e *Emitter) Csel(rd, rn, rm, cond int) {
	e.emit32(0x9A800000 | uint32(rm)<<16 | uint32(cond)<<12 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) LdrImm(rd, rn int, imm uint32) {
	e.emit32(0xF9400000 | (uint32(imm)&0xFFF)<<10 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) StrImm(rt, rn int, imm uint32) {
	e.emit32(0xF9000000 | (uint32(imm)&0xFFF)<<10 | uint32(rn)<<5 | uint32(rt))
}

func (e *Emitter) LdrReg(rd, rn, rm int) {
	e.emit32(0xF8607800 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) StrReg(rt, rn, rm int) {
	e.emit32(0xF8207800 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rt))
}

func (e *Emitter) MovImm16(rd int, val uint32, shift int) {
	e.emit32(0xD2800000 | uint32(shift/16)<<21 | (val&0xFFFF)<<5 | uint32(rd))
}

func (e *Emitter) MovImm64(rd int, val uint64) {
	e.Movz(rd, uint16(val), 0)
	if v := uint16(val >> 16); v != 0 {
		e.Movk(rd, v, 16)
	}
	if v := uint16(val >> 32); v != 0 {
		e.Movk(rd, v, 32)
	}
	if v := uint16(val >> 48); v != 0 {
		e.Movk(rd, v, 48)
	}
}

func (e *Emitter) Movz(rd int, imm uint16, shift int) {
	e.emit32(0xD2800000 | uint32(shift/16)<<21 | uint32(imm)<<5 | uint32(rd))
}

func (e *Emitter) Movk(rd int, imm uint16, shift int) {
	e.emit32(0xF2800000 | uint32(shift/16)<<21 | uint32(imm)<<5 | uint32(rd))
}

func (e *Emitter) Sbfx(rd, rn, lsb, width int) {
	immr := uint32(lsb) & 0x3F
	imms := uint32(lsb+width-1) & 0x3F
	e.emit32(0x93400000 | immr<<16 | imms<<10 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) ExtractUnsigned(rd, rn, width int) {
	imms := uint32(width - 1)
	e.emit32(0xD3400000 | imms<<10 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) And48(rd, rn int) {
	// AND Xd, Xn, #0x0000FFFFFFFFFFFF
	e.emit32(0x92000000 | 1<<22 | 16<<16 | 47<<10 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) Ret() {
	e.emit32(0xD65F03C0)
}

func (e *Emitter) Br(rn int) {
	e.emit32(0xD61F0000 | uint32(rn)<<5)
}

func (e *Emitter) Blr(rn int) {
	e.emit32(0xD63F0000 | uint32(rn)<<5)
}

func (e *Emitter) Bl(offset int) {
	e.emit32(0x94000000 | (uint32(offset) & 0x03FFFFFF))
}

func (e *Emitter) LdrW(rd, rn int, imm12 uint32) {
	e.emit32(0xB9400000 | (imm12&0xFFF)<<10 | uint32(rn)<<5 | uint32(rd))
}

func (e *Emitter) CbnzPlaceholder(rn int) int {
	pos := len(e.Code)
	e.emit32(0xB5000000 | uint32(rn))
	return pos
}

func (e *Emitter) PatchCbnz(offset, target int) {
	e.PatchCbz(offset, target)
}

func (e *Emitter) Cbnz(rn int, offset int) {
	e.emit32(0xB5000000 | (uint32(offset)&0x7FFFF)<<5 | uint32(rn))
}

func (e *Emitter) BPlaceholder() int {
	pos := len(e.Code)
	e.emit32(0x14000000)
	return pos
}

func (e *Emitter) CbzPlaceholder(rn int) int {
	pos := len(e.Code)
	e.emit32(0xB4000000 | uint32(rn))
	return pos
}

func (e *Emitter) PatchCbz(offset, target int) {
	imm := (target - offset) / 4
	inst := binary.LittleEndian.Uint32(e.Code[offset : offset+4])
	e.patch32(offset, (inst&0xFF00001F)|(uint32(imm)&0x7FFFF)<<5)
}

func (e *Emitter) BcondPlaceholder(cond int) int {
	pos := len(e.Code)
	e.emit32(0x54000000 | uint32(cond))
	return pos
}

func (e *Emitter) PatchB(offset, target int) {
	// B offset is scaled by 4
	imm := (target - offset) / 4
	e.patch32(offset, 0x14000000|(uint32(imm)&0x03FFFFFF))
}

func (e *Emitter) PatchBcond(offset, target int) {
	// B.cond offset is scaled by 4, bits 5-23
	imm := (target - offset) / 4
	inst := binary.LittleEndian.Uint32(e.Code[offset : offset+4])
	e.patch32(offset, (inst&0xFF00001F)|(uint32(imm)&0x3FFFF)<<5)
}

func (e *Emitter) patch32(offset int, inst uint32) {
	binary.LittleEndian.PutUint32(e.Code[offset:offset+4], inst)
}

func (e *Emitter) SaveCalleeSaved() {
	// Push X19-X28
	for i := 19; i <= 28; i++ {
		e.StrPre(i)
	}
}

func (e *Emitter) RestoreCalleeSaved() {
	// Pop X28-X19
	for i := 28; i >= 19; i-- {
		e.LdrPost(i)
	}
}

func (e *Emitter) SaveCallRegs() {
	e.StrPre(19)
	e.StrPre(20)
	e.StrPre(21)
	e.StrPre(22)
	e.StrPre(23)
	e.StrPre(29)
	e.StrPre(30)
}

func (e *Emitter) RestoreCallRegs() {
	e.LdrPost(30)
	e.LdrPost(29)
	e.LdrPost(23)
	e.LdrPost(22)
	e.LdrPost(21)
	e.LdrPost(20)
	e.LdrPost(19)
}

func (e *Emitter) StrPre(rt int) {
	e.emit32(0xF81F0FE0 | uint32(rt))
}

func (e *Emitter) LdrPost(rt int) {
	e.emit32(0xF84107E0 | uint32(rt))
}

func (e *Emitter) FaddD(fd, fn, fm int) {
	e.emit32(0x1E602800 | uint32(fm)<<16 | uint32(fn)<<5 | uint32(fd))
}

func (e *Emitter) FsubD(fd, fn, fm int) {
	e.emit32(0x1E603800 | uint32(fm)<<16 | uint32(fn)<<5 | uint32(fd))
}

func (e *Emitter) FmulD(fd, fn, fm int) {
	e.emit32(0x1E600800 | uint32(fm)<<16 | uint32(fn)<<5 | uint32(fd))
}

func (e *Emitter) FdivD(fd, fn, fm int) {
	e.emit32(0x1E601800 | uint32(fm)<<16 | uint32(fn)<<5 | uint32(fd))
}

func (e *Emitter) FnegD(fd, fn int) {
	e.emit32(0x1E614000 | uint32(fn)<<5 | uint32(fd))
}

func (e *Emitter) FcmpD(fn, fm int) {
	e.emit32(0x1E602000 | uint32(fm)<<16 | uint32(fn)<<5)
}

func (e *Emitter) FmovToFP(fd, rn int) {
	e.emit32(0x9E670000 | uint32(rn)<<5 | uint32(fd))
}

func (e *Emitter) FmovToGP(rd, fn int) {
	e.emit32(0x9E660000 | uint32(fn)<<5 | uint32(rd))
}

func (e *Emitter) IntToDouble(fd, rn int) {
	e.emit32(0x9E620000 | uint32(rn)<<5 | uint32(fd))
}

func (e *Emitter) DoubleToInt(rd, fn int) {
	e.emit32(0x9E780000 | uint32(fn)<<5 | uint32(rd))
}

func condToCode(cond string) uint32 {
	switch cond {
	case "EQ": return 0
	case "NE": return 1
	case "CS", "HS": return 2
	case "CC", "LO": return 3
	case "MI": return 4
	case "PL": return 5
	case "VS": return 6
	case "VC": return 7
	case "HI": return 8
	case "LS": return 9
	case "GE": return 10
	case "LT": return 11
	case "GT": return 12
	case "LE": return 13
	case "AL": return 14
	case "NV": return 15
	}
	return 0
}

func (e *Emitter) invertCond(cond uint32) uint32 {
	return cond ^ 1
}

