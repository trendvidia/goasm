# goasm

goasm turns short blocks of assembly into Go methods for a JIT compiler.
You write each instruction pattern once, as a named template:

```asm
# ALIAS BP = X22

# TEMPLATE LoadSlot(slot uint8)
    MOVZ X24, slot
    LDR  X24, [BP, X24, LSL #3]
```

and goasm generates a typed method on your `Emitter`:

```go
func (e *Emitter) EmitLoadSlot(slot uint8) {
	e.Movz(24, uint16(slot), 0)
	e.LdrReg(24, 22, 24)
}
```

Your JIT then calls `e.EmitLoadSlot(n)` instead of spelling out the
instruction sequence by hand. Templates for arm64 and amd64 produce methods
with the same names, so one compiler can drive both.

goasm does not encode machine code itself. Each assembly line becomes a call
to a method on an `Emitter` type that **you** provide for each architecture
(`Movz`, `LdrReg`, `AddReg`, ...), and that method writes the bytes. goasm's
part is the template language, register aliases, parameter substitution, and
keeping the generated methods in step with the templates.

## Status

goasm was extracted from the JIT of a bytecode VM and is used there. It is
small and young.

- **arm64** is the more complete encoder. `gen/testdata/jit/emitter.go` is
  a reference arm64 `Emitter` that the generated code compiles against.
- **amd64** is experimental. It accepts an Intel-style syntax mixed with
  arm64 mnemonics (`B.EQ`, `CSET`, `SBFX`), and there is no reference
  amd64 `Emitter` in this repository, so its output is checked as bytes
  only. Read the generated amd64 code before you rely on it.
- The generated file declares `package jit` unless you pass `-pkg`.

## Usage

Pin the command to a tag and run it from `go:generate` in the package that
holds your `Emitter`:

```go
//go:generate go run github.com/trendvidia/goasm/cmd/goasm@v1.1.0 -arch arm64 -in templates/arm64.goasm -out emit_arm64.go
//go:generate go run github.com/trendvidia/goasm/cmd/goasm@v1.1.0 -arch amd64 -in templates/amd64.goasm -out emit_amd64.go
```

Each output file carries a `//go:build <arch>` line, so both can sit in the
same package next to an `Emitter` per architecture. Keep the templates in the
repository that holds the generated files, and commit both.

[docs/design.md](docs/design.md) describes the template format, how
operands map to `Emitter` calls, and what the generator does not do yet.

## As a library

- `template.ParseFile` (or `template.NewParser` and `Parse`) reads a
  `.goasm` file into templates and register aliases.
- `gen.NewEncoder`, `gen.NewGenerator` and `Generator.Generate` write the
  Go source for one architecture.
- `gen.MethodName` is the naming rule (`Emit` + template name), exported so
  tools that link templates to generated code use the rule rather than a
  copy of it.

## Development

```sh
go test ./...      # includes the golden test over gen/testdata
go generate ./...  # regenerates gen/testdata/jit from gen/testdata/templates
```

The golden test regenerates `gen/testdata/jit/emit_*.go` from
`gen/testdata/templates/*.goasm` and requires byte-identical output, so any
change to generated code shows up as a fixture diff in review.

## License

MIT; see [LICENSE](LICENSE).
