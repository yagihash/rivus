package gossa

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testdata = "../../../testdata/go"

func TestLoad(t *testing.T) {
	p, err := Load(testdata, []string{"./cases/hello"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LoadErrors) != 0 {
		t.Fatalf("load errors: %v", p.LoadErrors)
	}
	var direct bool
	for _, fn := range p.Funcs {
		if FuncID(fn) == "example.com/cases/cases/hello.Direct" {
			direct = true
			if !p.Own[fn] {
				t.Errorf("%s should be own", FuncID(fn))
			}
		}
		// Dependencies have no bodies.
		if strings.HasPrefix(FuncID(fn), "example.com/cases/lib.") {
			t.Errorf("dependency function %s has a body", FuncID(fn))
		}
	}
	if !direct {
		t.Fatalf("Direct not found; funcs=%d", len(p.Funcs))
	}
}

func TestSnippetAndText(t *testing.T) {
	p, err := Load(testdata, []string{"./cases/hello"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range p.Funcs {
		if FuncID(fn) != "example.com/cases/cases/hello.Direct" {
			continue
		}
		var found bool
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Pos().IsValid() && strings.Contains(p.Snippet(in.Pos()), "lib.Source()") {
					found = true
					if got := p.Position(in.Pos()); got.Line != 6 {
						t.Errorf("line = %d, want 6", got.Line)
					}
					if got := p.ExprText(in.Pos()); !strings.Contains(got, "lib . Source ( )") {
						t.Errorf("ExprText = %q", got)
					}
				}
			}
		}
		if !found {
			t.Error("snippet for lib.Source() not found")
		}
		if bt := p.BodyText(fn); !strings.HasPrefix(bt, "func Direct ( )") {
			t.Errorf("BodyText = %q", bt)
		}
	}
}

func TestLoadErrorsAreRecorded(t *testing.T) {
	// A package that does not type-check must not fail the whole load.
	p, err := Load(testdata, []string{"./cases/hello", "./cases/doesnotexist"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LoadErrors) == 0 {
		t.Fatal("expected a recorded load error")
	}
}

func TestImplementations(t *testing.T) {
	p, err := Load(testdata, []string{"./cases/impl"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Implementations("(example.com/cases/cases/impl.Store).Save")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"(*example.com/cases/cases/impl.memStore).Save",
		"(example.com/cases/cases/impl.fileStore).Save",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}

	// A type of the universe has no package.
	if _, err := p.Implementations("(error).Error"); err != nil {
		t.Errorf("(error).Error: %v", err)
	}
	for _, bad := range []string{"os.Open", "(*impl.Store).Save", "(example.com/cases/cases/impl.Store).Nope", "(example.com/cases/cases/impl.memStore).Save", "(no/such.Pkg).M"} {
		if _, err := p.Implementations(bad); err == nil {
			t.Errorf("%q should not be accepted", bad)
		}
	}
}

// A package outside a module (a file named on the command line) has no Module, as the standard library
// has none: it is the code to analyse, so its bodies are built (yagihash/rivus #51).
func TestLoadOutsideModuleWithDeps(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n\nfunc Source() string { return \"x\" }\nfunc Sink(s string) {}\n\nfunc main() { Sink(Source()) }\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, deps := range []bool{false, true} {
		p, err := Load(dir, []string{"main.go"}, Options{Deps: deps})
		if err != nil {
			t.Fatal(err)
		}
		own := 0
		for fn := range p.Own {
			if len(fn.Blocks) > 0 {
				own++
			}
		}
		if own < 3 {
			t.Errorf("deps=%v: %d own functions with a body, want at least 3 (load errors: %v)", deps, own, p.LoadErrors)
		}
	}
}
