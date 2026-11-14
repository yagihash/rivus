package gossa

import (
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
