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
	// the content may be written from outside this function's stores (`mem::replace(&mut b, ..)`, `mem::swap`, a
	// function that assigns through the `&mut` it is given): unless every use of the object is a store into it or a
	// read of it as a writer, it is not known
	if !onlyRead(root.Func, root, 4, true) {
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
	if strings.Contains(held, "dyn ") {
		return ""
	}
	return held
}

// onlyRead reports whether every use, in fn, of the value root (and of its copies and conversions) leaves what it holds
// as it is: it is called as a writer (`w.write_all(p)`), passed to a function of the program that does the same
// (depth levels down), or, when own is set (root is the object made in fn), stored into. A use that may write
// something else into it, or keep it (a call without a body, a store of the object itself, a φ, a return) is
// not that: "when in doubt, it is not known".
func onlyRead(fn *sg.Func, root *sg.Value, depth int, own bool) bool {
	alias := map[*sg.Value]bool{root: true}
	for changed := true; changed; {
		changed = false
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if (in.Op == ir.OpCopy || in.Op == ir.OpConvert) && len(in.Args) == 1 && len(in.Dsts) == 1 && alias[in.Args[0]] && !alias[in.Dsts[0]] {
					alias[in.Dsts[0]] = true
					changed = true
				}
			}
		}
	}
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			for k, a := range in.Args {
				if !alias[a] {
					continue
				}
				switch in.Op {
				case ir.OpCopy, ir.OpConvert:
				case ir.OpStore:
					if !(own && k == 0) {
						return false
					}
				case ir.OpCall:
					if !callOnlyReads(in, k, depth) {
						return false
					}
				default:
					return false
				}
			}
		}
	}
	return true
}

// callOnlyReads: the call uses its operand k as a writer and does not change what it holds.
func callOnlyReads(in *sg.Instr, k, depth int) bool {
	if in.Callee != nil && k == 0 && (strings.HasPrefix(in.Callee.Symbol, "std::io::Write::") || strings.HasPrefix(in.Callee.Symbol, "std::fmt::Write::")) {
		return true
	}
	if in.Static != nil && depth > 0 && k < len(in.Static.Params) {
		return onlyRead(in.Static, in.Static.Params[k], depth-1, false)
	}
	return false
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
