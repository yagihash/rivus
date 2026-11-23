package ir

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func streamOfTwo(t *testing.T) []byte {
	t.Helper()
	fn := func(id string, file string) *Func {
		pos := Pos{File: file, Line: 1, Col: 1, Offset: 1, Length: 1}
		return &Func{ID: FuncID(id), Pos: pos, Blocks: []Block{{ID: 0, Term: Term{Kind: TermReturn, Pos: pos}}}}
	}
	p := &Program{Funcs: []*Func{fn("a.f", "a.go"), fn("b.g", "b.go")}, Impls: []Implementations{{Method: "M"}}}
	var buf bytes.Buffer
	if err := Write(&buf, p); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ReadFuncs gives the functions Read gives, in the order, and the same program without them.
func TestReadFuncsMatchesRead(t *testing.T) {
	stream := streamOfTwo(t)
	want, err := Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	var got []*Func
	p, err := ReadFuncs(bytes.NewReader(stream), func(f *Func) error {
		got = append(got, f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want.Funcs) {
		t.Errorf("the functions differ")
	}
	if p.Funcs != nil || !reflect.DeepEqual(p.Files, want.SourceFiles()) || !reflect.DeepEqual(p.Impls, want.Impls) {
		t.Errorf("the program differs: %+v, files %v want %v", p, p.Files, want.SourceFiles())
	}
}

func TestReadFuncsStopsOnTheError(t *testing.T) {
	stream := streamOfTwo(t)
	dup := append(append([]byte(nil), stream...), []byte(strings.SplitN(string(stream), "\n", 3)[1]+"\n")...)
	if _, err := ReadFuncs(bytes.NewReader(dup), func(*Func) error { return nil }); err == nil || !strings.Contains(err.Error(), "defined twice") {
		t.Errorf("a function defined twice: %v", err)
	}
}
