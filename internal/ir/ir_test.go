package ir

import (
	"bytes"
	"strings"
	"testing"
)

// sample is a function with a string built from a constant and a parameter,
// a branch, a call that may throw to a handler, and a recover root.
func sample() *Program {
	f := &Func{
		ID:      "example.com/m.Query",
		Params:  []Param{{Name: "id", Type: "string", Class: ClassString}},
		Results: []TypeID{"error"},
		Locals: []Local{
			{Name: "id", Type: "string"},           // 0
			{Type: "string"},                       // 1: the constant
			{Name: "q", Type: "string"},            // 2
			{Type: "bool"},                         // 3
			{Type: "error", Class: ClassInterface}, // 4
			{Type: "error", Class: ClassInterface}, // 5: the caught value
		},
		Blocks: []Block{
			{ID: 0, Instrs: []Instr{
				{Op: OpParam, Dsts: []LocalID{0}, Index: 0},
				{Op: OpConst, Dsts: []LocalID{1}, Const: &Const{Kind: ConstString, Value: "SELECT * FROM t WHERE id = "}},
				{Op: OpBinop, Dsts: []LocalID{2}, Args: []LocalID{1, 0}, Operator: "+"},
				{Op: OpBinop, Dsts: []LocalID{3}, Args: []LocalID{2, 1}, Operator: "=="},
			}, Term: Term{Kind: TermBranch, Cond: 3, CondPos: Pos{File: "q.go", Offset: 10, Length: 6}},
				Succs: []Edge{{1, EdgeTrue}, {2, EdgeFalse}}},
			{ID: 1, Instrs: []Instr{
				{Op: OpCall, Dsts: []LocalID{4}, Args: []LocalID{2},
					Callee: &Callee{Kind: CalleeStatic, Symbol: "database/sql.(*DB).Query"},
					ArgPos: []Pos{{File: "q.go", Offset: 30, Length: 1}}},
			}, Term: Term{Kind: TermReturn, Values: []LocalID{4}},
				Succs: []Edge{{3, EdgeException}}},
			{ID: 2, Term: Term{Kind: TermReturn, Values: []LocalID{4}}},
			{ID: 3, Instrs: []Instr{
				{Op: OpExceptionValue, Dsts: []LocalID{5}},
			}, Term: Term{Kind: TermReturn, Values: []LocalID{5}}},
		},
	}
	return &Program{Funcs: []*Func{f}, Impls: []Implementations{{Method: "io.Reader.Read", Impls: []FuncID{"m.R.Read"}}}}
}

func TestSampleIsValid(t *testing.T) {
	if err := sample().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestStreamRoundTrip(t *testing.T) {
	p := sample()
	var buf bytes.Buffer
	if err := Write(&buf, p); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 || lines[0] != `{"schema_version":"`+SchemaVersion+`"}` {
		t.Fatalf("stream:\n%s", buf.String())
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	if err := Write(&again, got); err != nil {
		t.Fatal(err)
	}
	var first bytes.Buffer
	Write(&first, p)
	if first.String() != again.String() {
		t.Errorf("the stream changed on a round trip:\n%s\n%s", first.String(), again.String())
	}
}

func TestReadRejectsWhatIsNotAStream(t *testing.T) {
	for name, in := range map[string]string{
		"empty":        ``,
		"no header":    `{"kind":"func","func":{"id":"f","blocks":[]}}`,
		"wrong schema": `{"schema_version":"rivus.ir/999"}`,
		"unknown rec":  `{"schema_version":"` + SchemaVersion + `"}` + "\n" + `{"kind":"what"}`,
		"unknown key":  `{"schema_version":"` + SchemaVersion + `"}` + "\n" + `{"kind":"func","func":{"id":"f","blocks":[{"id":0,"term":{"kind":"return"}}],"bogus":1}}`,
	} {
		if _, err := Read(strings.NewReader(in)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(f *Func)
		want   string
	}{
		{"local not declared", func(f *Func) { f.Blocks[0].Instrs[2].Args[0] = 99 }, "not declared"},
		{"binop with one argument", func(f *Func) { f.Blocks[0].Instrs[2].Args = f.Blocks[0].Instrs[2].Args[:1] }, "reads 1"},
		{"no operator", func(f *Func) { f.Blocks[0].Instrs[2].Operator = "" }, "no operator"},
		{"const without a value", func(f *Func) { f.Blocks[0].Instrs[1].Const = nil }, "no constant"},
		{"param out of range", func(f *Func) { f.Blocks[0].Instrs[0].Index = 3 }, "parameter 3"},
		{"edge to nowhere", func(f *Func) { f.Blocks[0].Succs[0].To = 17 }, "not a block"},
		{"edge to the entry", func(f *Func) { f.Blocks[2].Term = Term{Kind: TermJump}; f.Blocks[2].Succs = []Edge{{0, EdgeNormal}} }, "root block"},
		{"branch without false edge", func(f *Func) { f.Blocks[0].Succs = f.Blocks[0].Succs[:1] }, "true and one false"},
		{"return with a normal edge", func(f *Func) { f.Blocks[2].Succs = []Edge{{3, EdgeNormal}} }, "no normal edge"},
		{"exception edge without a call", func(f *Func) { f.Blocks[2].Succs = []Edge{{3, EdgeException}} }, "neither in a call"},
		{"block id", func(f *Func) { f.Blocks[1].ID = 5 }, "has the id"},
		{"unknown op", func(f *Func) { f.Blocks[0].Instrs[1].Op = "frobnicate" }, "unknown op"},
		{"call without callee", func(f *Func) { f.Blocks[1].Instrs[0].Callee = nil }, "no callee"},
		{"argument positions", func(f *Func) { f.Blocks[1].Instrs[0].ArgPos = []Pos{{}, {}} }, "argument positions"},
		{"entry is not a root", func(f *Func) { f.Roots = []BlockID{3} }, "entry block is not a root"},
		{"unknown type class", func(f *Func) { f.Locals[0].Class = "huge" }, "unknown type class"},
	}
	for _, c := range cases {
		p := sample()
		c.mutate(p.Funcs[0])
		err := p.Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error with %q", c.name, err, c.want)
		}
	}
}

func TestDynamicCallNeedsAReceiver(t *testing.T) {
	p := sample()
	in := &p.Funcs[0].Blocks[1].Instrs[0]
	in.Callee = &Callee{Kind: CalleeDynamic, Symbol: "io.Reader.Read"}
	in.Args = nil
	in.ArgPos = nil
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "receiver") {
		t.Errorf("got %v", err)
	}
}

func TestRecoverRoot(t *testing.T) {
	p := sample()
	f := p.Funcs[0]
	f.Roots = []BlockID{0, 3}
	// block 1 reaches 3 through an exception edge: a root must have no
	// predecessor.
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "root block") {
		t.Errorf("got %v", err)
	}
	f.Blocks[1].Succs = nil
	if err := p.Validate(); err != nil {
		t.Errorf("a root with no predecessor: %v", err)
	}
}

const wantFormat = `func example.com/m.Query(0:id) -> (error)
  b0:
    %0(id) = param 0
    %1 = const string "SELECT * FROM t WHERE id = "
    %2(q) = binop "+" %1, %0(id)
    %3 = binop "==" %2(q), %1
    branch %3  ; true->b1 false->b2
  b1:
    %4 = call static database/sql.(*DB).Query(%2(q))
    return %4  ; exception->b3
  b2:
    return %4
  b3:
    %5 = exception_value
    return %5
`

func TestFormat(t *testing.T) {
	got := sample().Funcs[0].Format()
	if got != wantFormat {
		t.Errorf("format:\n%s\nwant:\n%s", got, wantFormat)
	}
}

// The calls that stand for operations of the language (channels, iteration)
// have a fixed number of arguments and results; one of another shape is an
// invalid IR, not something for the engine to index into (#17).
func TestReservedCallsNeedTheirShape(t *testing.T) {
	cases := []struct {
		sym        string
		dsts, args int
		ok         bool
	}{
		{"chan.send", 0, 2, true}, {"chan.send", 0, 1, false}, {"chan.send", 0, 0, false},
		{"chan.recv", 1, 1, true}, {"chan.recv", 2, 1, true}, {"chan.recv", 0, 1, false}, {"chan.recv", 1, 0, false},
		{"iter.start", 1, 1, true}, {"iter.start", 0, 1, false}, {"iter.start", 1, 0, false},
		{"iter.next", 3, 1, true}, {"iter.next", 2, 1, false}, {"iter.next", 3, 0, false}, {"iter.next", 0, 0, false},
		{"iter.next_string", 3, 1, true}, {"iter.next_string", 0, 0, false},
	}
	for _, c := range cases {
		p := sample()
		in := &p.Funcs[0].Blocks[1].Instrs[0]
		in.Callee = &Callee{Kind: CalleeStatic, Symbol: c.sym}
		in.Dsts, in.Args, in.ArgPos = nil, nil, nil
		for i := 0; i < c.dsts; i++ {
			in.Dsts = append(in.Dsts, 4)
		}
		for i := 0; i < c.args; i++ {
			in.Args = append(in.Args, 2)
		}
		// the block returns local 4: keep it written
		if len(in.Dsts) == 0 {
			p.Funcs[0].Blocks[1].Instrs = append([]Instr{{Op: OpConst, Dsts: []LocalID{4}, Const: &Const{Kind: ConstNil}}}, p.Funcs[0].Blocks[1].Instrs...)
		}
		err := p.Validate()
		if c.ok && err != nil {
			t.Errorf("%s with %d results and %d arguments: %v", c.sym, c.dsts, c.args, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s with %d results and %d arguments is accepted", c.sym, c.dsts, c.args)
		}
	}
}
