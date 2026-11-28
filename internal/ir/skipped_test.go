package ir

import (
	"bytes"
	"reflect"
	"testing"
)

func TestFnSkippedRoundTripsThroughTheStream(t *testing.T) {
	p := &Program{FnSkipped: []FnSkipped{
		{Unit: "google_cloud_auth", Func: "google_cloud_auth::x::{closure#0}", Dependency: true, Why: "not implemented: FIXME(async_closures)"},
		{Unit: "mine", Func: "mine::f", Why: "a panic"},
	}}
	var buf bytes.Buffer
	if err := Write(&buf, p); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.FnSkipped, p.FnSkipped) {
		t.Errorf("got %+v, want %+v", got.FnSkipped, p.FnSkipped)
	}
}

// A position comes back from the stream as it went in. The stream leaves the file of the function out of the positions that are in it
// (Func.File), and the reader puts it back into the positions that have none. Two forms did not come back as they were: a position that
// has only a file (no offset, line or column) lost its file, and a position that has numbers but no file got the file of the function.
func TestPosRoundTripsThroughTheStream(t *testing.T) {
	mk := func(entry Pos, instr Pos) *Program {
		f := &Func{ID: "f", Pos: Pos{File: "a.go", Offset: 1, Line: 1, Col: 1}, Locals: []Local{{}}, Blocks: []Block{{
			ID: 0, Instrs: []Instr{{Op: OpConst, Dsts: []LocalID{0}, Const: &Const{Kind: ConstInt, Value: "1"}, Pos: instr}}, Term: Term{Kind: TermReturn, Pos: entry}}}}
		return &Program{Funcs: []*Func{f}}
	}
	for _, c := range []struct {
		name        string
		term, instr Pos
	}{
		{"every position of the function's file", Pos{File: "a.go", Offset: 5, Line: 2, Col: 1}, Pos{File: "a.go", Offset: 7, Line: 3, Col: 4}},
		{"a file only", Pos{File: "a.go"}, Pos{File: "a.go", Offset: 7, Line: 3, Col: 4}},
		{"another file", Pos{File: "b.go", Offset: 5, Line: 2, Col: 1}, Pos{File: "a.go", Offset: 7, Line: 3, Col: 4}},
		{"numbers and no file", Pos{File: "a.go", Offset: 5, Line: 2, Col: 1}, Pos{Offset: 7, Line: 2, Col: 3}},
		{"not set", Pos{}, Pos{File: "a.go", Offset: 7, Line: 3, Col: 4}},
	} {
		p := mk(c.term, c.instr)
		var buf bytes.Buffer
		if err := Write(&buf, p); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if p.Funcs[0].File != "" || p.Funcs[0].Pos.File != "a.go" {
			t.Errorf("%s: Write changed the function it was given: %+v", c.name, p.Funcs[0].Pos)
		}
		got, err := Read(&buf)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		g := got.Funcs[0]
		if g.Blocks[0].Term.Pos != c.term || g.Blocks[0].Instrs[0].Pos != c.instr || g.Pos != p.Funcs[0].Pos {
			t.Errorf("%s: term %+v instr %+v fn %+v; want term %+v instr %+v", c.name, g.Blocks[0].Term.Pos, g.Blocks[0].Instrs[0].Pos, g.Pos, c.term, c.instr)
		}
	}
}
