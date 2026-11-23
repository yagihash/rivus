package ir

import (
	"errors"
	"testing"
)

func memProgram() *Program {
	return &Program{Funcs: []*Func{
		{ID: "a.F", Symbol: "a.F"},
		{ID: "b.G", Dep: true},
		{ID: "a.F", Symbol: "a.F2"}, // the same ID twice: the number tells them apart
	}}
}

func TestMemSourceNamesFunctionsByIDAndNumber(t *testing.T) {
	p := memProgram()
	src := MemSource(p)
	if src.Len() != 3 || src.Header() != p {
		t.Fatalf("len %d", src.Len())
	}
	ref, ok := src.Lookup("a.F")
	if !ok || ref.Seq != 2 {
		t.Fatalf("Lookup gives the last function with the ID: %+v", ref)
	}
	if f, err := src.Func(Ref{ID: "a.F", Seq: 0}); err != nil || f != p.Funcs[0] {
		t.Fatalf("the first a.F: %v %v", f, err)
	}
	if _, err := src.Func(Ref{ID: "a.F", Seq: 1}); err == nil {
		t.Fatal("a number that holds another function is an error")
	}
	if _, err := src.Func(Ref{ID: "a.F", Seq: 9}); err == nil {
		t.Fatal("a number past the end is an error")
	}
	if h := src.Head(1); h.ID != "b.G" || !h.Dep {
		t.Fatalf("head %+v", h)
	}
}

func TestWalkProjectLeavesDependenciesUnread(t *testing.T) {
	src := MemSource(memProgram())
	var n int
	WalkProject(src, func(ref Ref, f *Func) bool { n++; return true })
	if n != 2 {
		t.Fatalf("project functions: %d", n)
	}
	n = 0
	WalkProject(src, func(ref Ref, f *Func) bool { n++; return false })
	if n != 1 {
		t.Fatalf("a reader that says stop is not called again: %d", n)
	}
}

func TestMemSourceDropAndRelease(t *testing.T) {
	p := memProgram()
	src := MemSource(p)
	src.Drop(Ref{ID: "a.F", Seq: 0})
	if !src.Head(0).Gone {
		t.Fatal("dropped function is not gone")
	}
	if _, err := src.Func(Ref{ID: "a.F", Seq: 0}); err == nil {
		t.Fatal("a dropped function is not read")
	}
	var n int
	src.Each(func(Ref, *Func) error { n++; return nil })
	if n != 2 {
		t.Fatalf("each: %d", n)
	}
	src.Release()
	if src.Len() != 0 || p.Funcs != nil {
		t.Fatal("release leaves the functions")
	}
}

func TestMustPanicsWithReadError(t *testing.T) {
	defer func() {
		var re *ReadError
		r := recover()
		e, _ := r.(error)
		if !errors.As(e, &re) || re.Ref.ID != "x" {
			t.Fatalf("recovered %v", r)
		}
	}()
	Must(MemSource(memProgram()), Ref{ID: "x", Seq: 5})
}
