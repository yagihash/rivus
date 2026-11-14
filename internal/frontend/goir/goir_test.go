package goir

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/testutil"
)

func extract(t *testing.T, patterns ...string) *ir.Program {
	t.Helper()
	p, err := gossa.Load(testutil.Testdata(), patterns, gossa.Options{NaiveForm: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LoadErrors) != 0 {
		t.Fatalf("load errors: %v", p.LoadErrors)
	}
	out, err := Extract(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func funcText(t *testing.T, p *ir.Program, name string) string {
	t.Helper()
	for _, f := range p.Funcs {
		if string(f.ID) == "example.com/cases/cases/irshape."+name {
			return f.Format()
		}
	}
	t.Fatalf("no function %s", name)
	return ""
}

func TestTestdataIsValidAndSurvivesTheStream(t *testing.T) {
	p := extract(t, "./...")
	if len(p.Funcs) < 50 {
		t.Fatalf("only %d functions", len(p.Funcs))
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ir.Write(&buf, p); err != nil {
		t.Fatal(err)
	}
	first := buf.String()
	back, err := ir.Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	ir.Write(&again, back)
	if first != again.String() {
		t.Error("the stream changed on a round trip")
	}
	for _, f := range p.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpUnsupported {
					t.Errorf("%s: unsupported: %s", f.ID, in.Reason)
				}
			}
		}
	}
}

// What the IR keeps of a query built by hand: the text of the constant, and
// the way the variable gets into it.
func TestStringInterpolationKeepsTheConstantAndTheVariable(t *testing.T) {
	p := extract(t, "./cases/irshape")
	const sql = `const string "SELECT * FROM users WHERE id = "`
	if got := funcText(t, p, "Concat"); !strings.Contains(got, sql) || !strings.Contains(got, `binop "+"`) {
		t.Errorf("Concat:\n%s", got)
	}
	if got := funcText(t, p, "Sprintf"); !strings.Contains(got, `const string "SELECT * FROM users WHERE id = %s"`) || !strings.Contains(got, "call static fmt.Sprintf") {
		t.Errorf("Sprintf:\n%s", got)
	}
	got := funcText(t, p, "Builder")
	if strings.Count(got, "(*strings.Builder).WriteString") != 2 || !strings.Contains(got, sql) {
		t.Errorf("Builder:\n%s", got)
	}
}

func TestFieldsAndElements(t *testing.T) {
	p := extract(t, "./cases/irshape")
	got := funcText(t, p, "Fields")
	if !strings.Contains(got, "field_store .a") || !strings.Contains(got, "field_load .b") || strings.Contains(got, "addr_of") {
		t.Errorf("Fields (the address is folded into the store and the load):\n%s", got)
	}
	got = funcText(t, p, "EscapingField")
	if !strings.Contains(got, "addr_of") || !strings.Contains(got, ".a") || strings.Contains(got, "field_store") {
		t.Errorf("EscapingField (the address is a value):\n%s", got)
	}
	got = funcText(t, p, "Elements")
	for _, want := range []string{"index_store", "index_load"} {
		if !strings.Contains(got, want) {
			t.Errorf("Elements lacks %s:\n%s", want, got)
		}
	}
	// v, ok := m["k"] writes the value and the flag in one instruction.
	found := false
	for _, f := range p.Funcs {
		if strings.HasSuffix(string(f.ID), "irshape.Elements") {
			for _, b := range f.Blocks {
				for _, in := range b.Instrs {
					if in.Op == ir.OpIndexLoad && len(in.Dsts) == 2 {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Error("no comma-ok index_load")
	}
}

func TestShortCircuitHasNoPhi(t *testing.T) {
	p := extract(t, "./cases/irshape")
	got := funcText(t, p, "ShortCircuit")
	if strings.Contains(got, "phi") || !strings.Contains(got, "copy") {
		t.Errorf("ShortCircuit:\n%s", got)
	}
}

func TestRecoverBlockIsARoot(t *testing.T) {
	p := extract(t, "./cases/irshape")
	for _, f := range p.Funcs {
		if string(f.ID) == "example.com/cases/cases/irshape.Recovers" {
			if len(f.Roots) != 2 {
				t.Errorf("roots %v", f.Roots)
			}
			if !strings.Contains(f.Format(), "throw") {
				t.Errorf("no throw:\n%s", f.Format())
			}
			return
		}
	}
	t.Fatal("no Recovers")
}

func TestClosuresGlobalsAndChannels(t *testing.T) {
	p := extract(t, "./cases/irshape")
	if got := funcText(t, p, "Counter"); !strings.Contains(got, "make_closure example.com/cases/cases/irshape.Counter$1") {
		t.Errorf("Counter:\n%s", got)
	}
	for _, f := range p.Funcs {
		if string(f.ID) == "example.com/cases/cases/irshape.Counter$1" {
			captured := 0
			for _, pa := range f.Params {
				if pa.Captured {
					captured++
				}
			}
			if captured != 1 {
				t.Errorf("captured parameters: %d", captured)
			}
		}
	}
	if got := funcText(t, p, "SetG"); !strings.Contains(got, "global_store example.com/cases/cases/irshape.G") {
		t.Errorf("SetG:\n%s", got)
	}
	if got := funcText(t, p, "GetG"); !strings.Contains(got, "global_load example.com/cases/cases/irshape.G") {
		t.Errorf("GetG:\n%s", got)
	}
	got := funcText(t, p, "Channels")
	for _, want := range []string{"go/chan.send", "go/chan.select", "go/chan.recv"} {
		if !strings.Contains(got, want) {
			t.Errorf("Channels lacks %s:\n%s", want, got)
		}
	}
}

func TestRangesAssertsAndCalls(t *testing.T) {
	p := extract(t, "./cases/irshape")
	if got := funcText(t, p, "Ranges"); !strings.Contains(got, "go/range.next") || !strings.Contains(got, "go/range.start") {
		t.Errorf("Ranges:\n%s", got)
	}
	if got := funcText(t, p, "Asserts"); strings.Count(got, "type_assert string") != 2 {
		t.Errorf("Asserts:\n%s", got)
	}
	got := funcText(t, p, "Dynamic")
	if !strings.Contains(got, "call dynamic io.Reader.Read") && !strings.Contains(got, "call dynamic (io.Reader).Read") {
		t.Errorf("no dynamic call:\n%s", got)
	}
	if !strings.Contains(got, "call value") || !strings.Contains(got, "[go]") {
		t.Errorf("no call of a value, or no go flag:\n%s", got)
	}
}

func TestImplementationsRecord(t *testing.T) {
	p, err := gossa.Load(testutil.Testdata(), []string{"./cases/impl"}, gossa.Options{NaiveForm: true})
	if err != nil {
		t.Fatal(err)
	}
	// Any interface method in the package will do; take the first dynamic call.
	var sym string
	prog, _ := Extract(p, Options{})
	for _, f := range prog.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpCall && in.Callee.Kind == ir.CalleeDynamic && sym == "" {
					sym = in.Callee.Symbol
				}
			}
		}
	}
	if sym == "" {
		t.Skip("no dynamic call in the impl case")
	}
	out, err := Extract(p, Options{Implementations: []string{sym}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Impls) != 1 || out.Impls[0].Method != sym {
		t.Errorf("implementations: %+v", out.Impls)
	}
}
