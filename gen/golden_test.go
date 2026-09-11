package gen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/trendvidia/goasm/gen"
	"github.com/trendvidia/goasm/template"
)

// TestGoldenEmitters regenerates jit/emit_<arch>.go from templates/<arch>.goasm
// and requires the bytes to match what is committed: the generator's output
// is the contract, and this is what makes a refactor of it provable.
func TestGoldenEmitters(t *testing.T) {
	for _, arch := range []string{"arm64", "amd64"} {
		templates, aliases, err := template.ParseFile(filepath.Join("..", "templates", arch+".goasm"))
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
		want, err := os.ReadFile(filepath.Join("..", "jit", "emit_"+arch+".go"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Errorf("%s: generated output differs from the committed jit/emit_%s.go; run go generate ./... and review", arch, arch)
		}
		for _, tmpl := range templates {
			if !bytes.Contains(want, []byte("func (e *Emitter) "+gen.MethodName(tmpl)+"(")) {
				t.Errorf("%s: %s has no method %s in the generated file", arch, tmpl.Name, gen.MethodName(tmpl))
			}
		}
	}
}
