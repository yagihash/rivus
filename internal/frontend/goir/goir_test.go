package goir

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/testutil"
)

func extract(t *testing.T, patterns ...string) *ir.Program {
	t.Helper()
	p, err := Load(testutil.Testdata(), patterns, gossa.Options{})
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
	for _, want := range []string{"chan.send", "chan.recv"} {
		if !strings.Contains(got, want) {
			t.Errorf("Channels lacks %s:\n%s", want, got)
		}
	}
}

func TestRangesAssertsAndCalls(t *testing.T) {
	p := extract(t, "./cases/irshape")
	if got := funcText(t, p, "Ranges"); !strings.Contains(got, "iter.next") || !strings.Contains(got, "iter.start") {
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
	p, err := Load(testutil.Testdata(), []string{"./cases/impl"}, gossa.Options{})
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
	out, err := Extract(p, Options{NoCandidates: true, Implementations: []string{sym}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Impls) != 1 || out.Impls[0].Method != sym {
		t.Errorf("implementations: %+v", out.Impls)
	}
}

// The text of a call is found by the position the IR gives, also when the file was not compiled as it
// is on disk: a `//line` directive, and cgo (the generated file is what is compiled; yagihash/rivus #55).
func TestTextsOfRelocatedFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/reloc\n\ngo 1.22\n")
	write("sink.go", "package reloc\n\nfunc Sink(s string) {}\n")
	write("line.go", "package reloc\n\n//line gram.y:100\nfunc FromLine(a string) {\n\tSink(a)\n}\n")
	write("cgo.go", "package reloc\n\n/*\nstatic int one(void) { return 1; }\n*/\nimport \"C\"\n\nfunc FromCgo(a string) {\n\t_ = C.one()\n\tSink(a)\n}\n")
	p, err := Load(dir, []string{"."}, gossa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LoadErrors) != 0 {
		t.Skipf("the package does not load here (cgo?): %v", p.LoadErrors)
	}
	prog, err := Extract(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	texts := NewTexts(p)
	seen := map[string]bool{}
	for _, f := range prog.Funcs {
		name := string(f.ID)
		if !strings.HasSuffix(name, ".FromLine") && !strings.HasSuffix(name, ".FromCgo") {
			continue
		}
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpCall || in.Callee == nil || !strings.HasSuffix(in.Callee.Symbol, ".Sink") {
					continue
				}
				seen[name] = true
				if got := texts.Snippet(in.Pos); !strings.Contains(got, "Sink(a)") {
					t.Errorf("%s: snippet = %q (position %+v)", name, got, in.Pos)
				}
				if got := texts.Expr(in.Pos); got == "" {
					t.Errorf("%s: no expression text (position %+v)", name, in.Pos)
				}
				if strings.Contains(in.Pos.File, "go-build") || filepath.IsAbs(in.Pos.File) {
					t.Errorf("%s: the file is %q", name, in.Pos.File)
				}
			}
		}
	}
	if len(seen) != 2 {
		t.Errorf("calls found in %v", seen)
	}
}

// G-15: a closure that captures values is made by an instruction that go/ssa
// gives no position (MakeClosure has none); the position of the function
// literal stands for it, so a reference to the closure has a place.
func TestMakeClosureHasAPosition(t *testing.T) {
	p := extract(t, "./...")
	n := 0
	for _, f := range p.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpMakeClosure || len(in.Args) == 0 {
					continue
				}
				n++
				if in.Pos.File == "" || in.Pos.Line == 0 {
					t.Errorf("%s: make_closure of %s has no position", f.ID, in.Func)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no closure with captured values in the testdata")
	}
}

// charReadOps lists, for the function, the instructions (index_load, call) that carry char_read, as "op symbol".
func charReadOps(t *testing.T, p *ir.Program, name string) (marked, all []string) {
	t.Helper()
	for _, f := range p.Funcs {
		if string(f.ID) != "example.com/cases/cases/irshape."+name {
			continue
		}
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpIndexLoad && in.Op != ir.OpCall {
					continue
				}
				s := string(in.Op)
				if in.Callee != nil {
					s += " " + in.Callee.Symbol
				}
				all = append(all, s)
				if in.CharRead {
					marked = append(marked, s)
				}
			}
		}
		return marked, all
	}
	t.Fatalf("no function %s", name)
	return
}

func TestCharReadIsMarkedOnTheElementOfAString(t *testing.T) {
	p := extract(t, "./cases/irshape")
	marked, _ := charReadOps(t, p, "CharReads")
	got := strings.Join(marked, ",")
	if got != "index_load,call iter.next_string" && got != "call iter.next_string,index_load" {
		t.Errorf("CharReads: marked = %q, want the s[i] index_load and the iter.next_string call", got)
	}
	marked, all := charReadOps(t, p, "NotCharReads")
	if len(marked) != 0 || len(all) == 0 {
		t.Errorf("NotCharReads: marked = %v of %v, want none marked (a map, a slice, len)", marked, all)
	}
}
