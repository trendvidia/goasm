package gen_test

//go:generate go run ../cmd/goasm -arch arm64 -in testdata/templates/arm64.goasm -out testdata/jit/emit_arm64.go
//go:generate go run ../cmd/goasm -arch amd64 -in testdata/templates/amd64.goasm -out testdata/jit/emit_amd64.go

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/trendvidia/goasm/gen"
	"github.com/trendvidia/goasm/template"
)

// TestGoldenEmitters regenerates testdata/jit/emit_<arch>.go from
// testdata/templates/<arch>.goasm and requires the bytes to match what is
// committed: the generator's output is the contract, and this is what makes
// a refactor of it provable.
func TestGoldenEmitters(t *testing.T) {
	for _, arch := range []string{"arm64", "amd64"} {
		templates, aliases, err := template.ParseFile(filepath.Join("testdata", "templates", arch+".goasm"))
		if err != nil {
			t.Fatalf("%s: %v", arch, err)
		}
		enc, err := gen.NewEncoder(arch, aliases)
		if err != nil {
			t.Fatal(err)
		}
		var got bytes.Buffer
		if err := gen.NewGenerator(arch, enc).Generate(&got, templates); err != nil {
			t.Fatalf("%s: %v", arch, err)
		}
		want, err := os.ReadFile(filepath.Join("testdata", "jit", "emit_"+arch+".go"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Errorf("%s: generated output differs from the committed testdata/jit/emit_%s.go; run go generate ./... and review", arch, arch)
		}
		for _, tmpl := range templates {
			if !bytes.Contains(want, []byte("func (e *Emitter) "+gen.MethodName(tmpl)+"(")) {
				t.Errorf("%s: %s has no method %s in the generated file", arch, tmpl.Name, gen.MethodName(tmpl))
			}
		}
	}
}

// TestARM64EmitterTypeChecks type-checks the generated arm64 emitters
// against testdata/jit/emitter.go, the reference arm64 Emitter, on any host:
// the golden test proves the bytes, this proves they compile against the
// method set they call. amd64 has no reference Emitter here, so its output
// is covered by the golden test alone.
func TestARM64EmitterTypeChecks(t *testing.T) {
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range []string{"emitter.go", "emit_arm64.go"} {
		f, err := parser.ParseFile(fset, filepath.Join("testdata", "jit", name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	conf := types.Config{
		Importer: importer.ForCompiler(fset, "source", nil),
		Sizes:    types.SizesFor("gc", "arm64"),
	}
	if _, err := conf.Check("jit", fset, files, nil); err != nil {
		t.Fatal(err)
	}
}
