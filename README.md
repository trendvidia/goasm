# goasm

A code generator for JIT emitters: `# TEMPLATE Name(params)` blocks of
assembly in `templates/*.goasm` compile to Go methods `(*Emitter).EmitName`
in `jit/emit_<arch>.go`, one file per architecture. Design in
[docs/goasm-design.md](docs/goasm-design.md).

## Using it

Pin the tool to a tag in the consuming module and run it from `go:generate`:

```go
//go:generate go run github.com/trendvidia/goasm/cmd/goasm@v1.1.0 -arch arm64 -in templates/arm64.goasm -out emit_arm64.go
//go:generate go run github.com/trendvidia/goasm/cmd/goasm@v1.1.0 -arch amd64 -in templates/amd64.goasm -out emit_amd64.go
```

Keep the templates in the repository that holds the generated files.

## As a library

- `template.ParseFile` / `template.NewParser` parse a `.goasm` file into
  templates and register aliases.
- `gen.NewEncoder`, `gen.NewGenerator` and `Generator.Generate` produce the
  emitter source; `gen.MethodName` is the naming rule (`Emit` + template
  name), exported so a consumer that links templates to generated code
  follows it rather than a copy.

`gen`'s golden test regenerates this repository's own `jit/emit_*.go` from
`templates/` and requires byte-identical output.
