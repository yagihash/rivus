package taint

import (
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// wrapperNews are the constructors of the wrappers of the standard library that hold one value and hand a call to
// it (`impl<W: Write + ?Sized> Write for Box<W>` calls `W::write`). The extractor drops the type arguments of a type
// (`std::boxed::Box`), but the ID of the constructor has them (`std::boxed::Box::<T>::new`): what a wrapper made
// right there holds is read from it.
var wrapperNews = []string{
	"std::boxed::Box::<", "std::rc::Rc::<", "std::sync::Arc::<", "std::cell::RefCell::<", "std::sync::Mutex::<",
	"std::sync::RwLock::<", "std::io::BufWriter::<", "std::io::LineWriter::<",
}

// heldType is the type that a wrapper of the standard library (a `Box`, an `Rc`, a `RefCell`, a `BufWriter`) that was made
// right there holds, when the function stored one value made by `Wrapper::<T>::new` in it, and nothing else ("" when
// it is not known: the wrapper came from outside, or was written into with something else). The type is that of the
// innermost value (`Rc<RefCell<T>>` holds `T`), for it is the one whose implementation a call through the wrapper reaches.
func heldType(v *sg.Value) string {
	root := allocRoot(v)
	if root == nil || root.Func == nil {
		return ""
	}
	held := ""
	for _, b := range root.Func.Blocks {
		for _, in := range b.Instrs {
			if in.Op != ir.OpStore || len(in.Args) != 2 || allocRoot(in.Args[0]) != root {
				continue
			}
			t := newType(in.Args[1])
			if t == "" || (held != "" && held != t) {
				return "" // written into with something that is not a constructor, or with two types: not known
			}
			held = t
		}
	}
	return held
}

// allocRoot is the object made right there (an alloc) that a value is a copy of, or nil.
func allocRoot(v *sg.Value) *sg.Value {
	for i := 0; i < 8 && v != nil; i++ {
		if v.Kind != sg.KResult || v.Instr == nil {
			return nil
		}
		switch v.Instr.Op {
		case ir.OpAlloc:
			return v
		case ir.OpCopy:
			if len(v.Instr.Args) != 1 {
				return nil
			}
			v = v.Instr.Args[0]
		default:
			return nil
		}
	}
	return nil
}

// newType is the type argument of the constructor that made a value (`T` of `std::boxed::Box::<T>::new`), peeled of the
// wrappers inside (`RefCell<T>` is `T`).
func newType(v *sg.Value) string {
	if v == nil || v.Kind != sg.KResult || v.Instr == nil || v.Instr.Op != ir.OpCall {
		return ""
	}
	id := string(v.Instr.StaticID)
	if !strings.HasSuffix(id, ">::new") {
		return ""
	}
	for _, p := range wrapperNews {
		if strings.HasPrefix(id, p) {
			return peelWrappers(id[len(p) : len(id)-len(">::new")])
		}
	}
	return ""
}

// peelWrappers takes `T` out of `Box<RefCell<T>>`, as far as the type is one of the wrappers (the type of a program is
// returned as it is).
func peelWrappers(t string) string {
	for i := 0; i < 8; i++ {
		j := strings.IndexByte(t, '<')
		if j < 0 || !strings.HasSuffix(t, ">") || !isWrapperName(t[:j]) {
			return t
		}
		t = t[j+1 : len(t)-1]
	}
	return ""
}

func isWrapperName(n string) bool {
	if !isStdType(n) {
		return false
	}
	for _, w := range []string{"boxed::Box", "rc::Rc", "sync::Arc", "cell::RefCell", "sync::Mutex", "sync::RwLock", "io::BufWriter", "io::LineWriter"} {
		if strings.HasSuffix(n, "::"+w) {
			return true
		}
	}
	return false
}
