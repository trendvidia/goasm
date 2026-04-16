# goasm — Go Code Generator for JIT Emitters

## Problem

GoVM's JIT emits ARM64 machine code via hand-written bit manipulation:

```go
// arm64.go — 500 lines of manual encoding
func (e *Emitter) AddReg(rd, rn, rm int) {
    e.emit32(0x8B000000 | uint32(rm)<<16 | uint32(rn)<<5 | uint32(rd))
}
```

```go
// compiler.go — verbose, error-prone opcode handlers
c.e.LdrImm(X24, RegBP, uint32(slot))
c.e.LdrImm(X25, RegBP, uint32(slot2))
c.e.AddReg(X24, X24, X25)
c.e.SubReg(X24, X24, X28)
c.e.StrImm(X24, RegSP, 0)
c.e.AddImm(RegSP, RegSP, ValueSize)
```

This approach doesn't scale to:
- **Two architectures** (ARM64 + x86-64) — doubles the emitter code
- **Register-based bytecode** — rewrites all ~40 opcode handlers
- **Tier 2 JIT** — many more instruction patterns

## Solution

A `go:generate` tool that compiles assembly templates into Go emitter functions:

```
Input:  templates/*.goasm (assembly + Go markers)
Output: jit/emit_arm64.go, jit/emit_amd64.go
```

## Template Format

```asm
# templates/arm64.goasm

# TEMPLATE AddLocalsInt(dst, src1, src2 uint8)
    LDR  X24, [BP, src1, LSL #3]
    LDR  X25, [BP, src2, LSL #3]
    ADD  X24, X24, X25
    SUB  X24, X24, X28           # X28 = tagInt constant
    STR  X24, [BP, dst, LSL #3]

# TEMPLATE IncrLocalInt(slot uint8, constIdx uint16)
    LDR  X24, [BP, slot, LSL #3]
    LDR  X25, [CONST, constIdx, LSL #3]
    ADD  X24, X24, X25
    SUB  X24, X24, X28
    STR  X24, [BP, slot, LSL #3]

# TEMPLATE JumpBackEdge(offset int16)
    B    offset                  # relative branch

# TEMPLATE PushInt(reg int)
    STR  reg, [SP]
    ADD  SP, SP, #8

# TEMPLATE PopTwo(rd1, rd2 int)
    SUB  SP, SP, #8
    LDR  rd2, [SP]
    SUB  SP, SP, #8
    LDR  rd1, [SP]
```

Same templates for x86-64:
```asm
# templates/amd64.goasm

# TEMPLATE AddLocalsInt(dst, src1, src2 uint8)
    MOV  RAX, [RBP + src1*8]
    ADD  RAX, [RBP + src2*8]
    SUB  RAX, tagIntConst
    MOV  [RBP + dst*8], RAX

# TEMPLATE IncrLocalInt(slot uint8, constIdx uint16)
    MOV  RAX, [RBP + slot*8]
    ADD  RAX, [R13 + constIdx*8]    # R13 = const base
    SUB  RAX, tagIntConst
    MOV  [RBP + slot*8], RAX
```

## Generated Output

For each template, goasm generates a typed Go function:

```go
// jit/emit_arm64.go (auto-generated — DO NOT EDIT)

func (e *Emitter) EmitAddLocalsInt(dst, src1, src2 uint8) {
    e.emit32(0xF9400300 | uint32(src1)<<10 | uint32(RegBP)<<5 | X24)
    e.emit32(0xF9400300 | uint32(src2)<<10 | uint32(RegBP)<<5 | X25)
    e.emit32(0x8B190318)  // ADD X24, X24, X25
    e.emit32(0xCB1C0318)  // SUB X24, X24, X28
    e.emit32(0xF9000300 | uint32(dst)<<10 | uint32(RegBP)<<5 | X24)
}
```

```go
// jit/emit_amd64.go (auto-generated — DO NOT EDIT)

func (e *Emitter) EmitAddLocalsInt(dst, src1, src2 uint8) {
    e.emitREX(1, 0, 0, 0)
    e.emit8(0x8B) // MOV RAX, [RBP + src1*8]
    e.emitModRM(...)
    // ...
}
```

The JIT compiler (`compiler.go`) calls the same API regardless of architecture:

```go
case bytecode.AddLocalsInt:
    dst, src1, src2 := code[ip], code[ip+1], code[ip+2]
    ip += 3
    c.e.EmitAddLocalsInt(dst, src1, src2)
```

## Architecture

```
templates/
    arm64.goasm        # ARM64 assembly templates
    amd64.goasm        # x86-64 assembly templates

cmd/goasm/
    main.go            # go:generate entry point
    parse.go           # template parser (~200 lines)
    arm64_enc.go       # ARM64 instruction encoder (~300 lines)
    amd64_enc.go       # x86-64 instruction encoder (~400 lines)
    codegen.go         # Go code generator (~100 lines)

jit/
    emit_arm64.go      # generated
    emit_amd64.go      # generated
    compiler.go        # unchanged — calls EmitXxx()
    arm64.go           # keep for non-template helpers (SaveCalleeSaved, etc.)
    amd64.go           # new: non-template helpers for x86-64
```

## Instruction Encoding

The tool needs to encode assembly text into machine code bytes. This is the core complexity:

### ARM64 (~30 instruction forms)
ARM64 is regular — fixed 32-bit instructions with predictable field layouts:
- Data processing: `ADD/SUB/MUL/SDIV` — `sf|opc|shift|Rm|imm6|Rn|Rd`
- Load/Store: `LDR/STR` — `size|opc|imm12|Rn|Rt`
- Branch: `B/B.cond/BL/CBZ` — `opc|imm26` or `opc|imm19|cond`
- Bitfield: `SBFX/UBFM/BFI` — `sf|opc|N|immr|imms|Rn|Rd`

Each form is a function that packs fields into a uint32. We already have most of these in `arm64.go` — goasm extracts them.

### x86-64 (~40 instruction forms)
x86-64 is irregular — variable-length instructions with prefixes, REX bytes, ModRM, SIB:
- REX prefix: `0100WRXB` for 64-bit operand size
- Opcode: 1-3 bytes
- ModRM: `mod|reg|rm` for register/memory addressing
- SIB: `scale|index|base` for complex addressing
- Displacement: 0/1/4 bytes
- Immediate: 0/1/2/4/8 bytes

More complex but well-documented. The encoder handles ~15 common patterns:
- `reg, reg` (register-register)
- `reg, [reg + disp]` (register-memory)
- `reg, imm` (register-immediate)
- `[reg + disp], reg` (memory-register)
- Branch targets (relative offsets)

## Register Conventions

### ARM64 (current)
```
X19 = JITContext pointer
X20 = stack base
X21 = SP (stack pointer as byte address)
X22 = BP (base pointer as byte address)
X23 = const cache base
X24-X27 = scratch
X28 = tagInt constant
```

### x86-64 (proposed)
```
RBX = JITContext pointer (callee-saved)
R12 = stack base (callee-saved)
R13 = const cache base (callee-saved)
R14 = BP as byte address (callee-saved)
R15 = SP as byte address (callee-saved)
RAX, RCX, RDX, RSI, RDI = scratch
R8-R11 = scratch
```

x86-64 has fewer callee-saved registers, so more save/restore in prologue/epilogue. But it has more scratch registers available per instruction.

## Template Syntax Details

```
# Comment line

# TEMPLATE name(param1 type, param2 type, ...)
    MNEMONIC operand1, operand2    # inline comment
    MNEMONIC operand1, operand2
    ...

# Labels (for internal branches within a template)
.loop:
    ...
    B.LT .loop

# Architecture-specific register aliases (defined in header)
# ALIAS BP = X22         (ARM64)
# ALIAS BP = R14         (x86-64)
# ALIAS SP = X21         (ARM64)
# ALIAS SP = R15         (x86-64)
# ALIAS CONST = X23      (ARM64)
# ALIAS CONST = R13      (x86-64)
```

Parameters can appear anywhere an immediate or register index would:
- `LDR X24, [BP, slot, LSL #3]` — `slot` is substituted as a uint8 shifted left
- `MOV RAX, [RBP + slot*8]` — same semantics on x86-64
- `B offset` — `offset` is a relative branch displacement

## Implementation Plan

### Phase 1: ARM64 only (~3 days)
1. Template parser (TEMPLATE/ALIAS directives, instruction lines)
2. ARM64 encoder (reuse field packing from existing `arm64.go`)
3. Go code generator (emit functions with `emit32` calls)
4. Migrate 5-10 existing opcode handlers to templates
5. Verify: `go generate ./...` produces identical output to current code

### Phase 2: x86-64 (~5 days)
1. x86-64 encoder (REX, ModRM, SIB, displacement)
2. x86-64 templates for the same 5-10 opcodes
3. x86-64 JIT context and memory allocation (`mmap` with `PROT_EXEC`)
4. Verify: basic arithmetic loop runs on x86-64

### Phase 3: Full migration (~3 days)
1. Migrate all ~40 opcode handlers to templates
2. Remove hand-written emitter functions from `arm64.go`
3. Both architectures share `compiler.go` (calls `EmitXxx`)

## Estimated Size

| Component | Lines |
|-----------|-------|
| Template parser | ~200 |
| ARM64 encoder | ~300 |
| x86-64 encoder | ~400 |
| Go code generator | ~100 |
| ARM64 templates | ~300 |
| x86-64 templates | ~350 |
| **Total** | **~1650** |

Compare to maintaining two hand-written emitters: ~500 (ARM64) + ~600 (x86-64) + duplication in compiler.go = ~1500 lines that must be kept in sync manually.

goasm pays for itself immediately when the second architecture is added, and saves significant effort during the register-based bytecode rewrite (Phase 1 of roadmap).

## Dependencies

None. Pure Go, no CGo, no external tools. The template files are plain text. The generator runs via `go generate`.

## Prior Art

- **DynASM** (LuaJIT) — C preprocessor that mixes assembly into C. Supports x86, x86-64, ARM, ARM64, MIPS, PPC. ~2000 lines of Lua. Our tool is similar in concept but Go-native and simpler (no preprocessor, just code generation).
- **asmjit** (C++) — JIT assembler library with builder API. Higher-level than DynASM. Too complex for our needs.
- **Cranelift** (Rust) — Full optimizing code generator. Way too heavy — it's a Tier 2 JIT by itself.
