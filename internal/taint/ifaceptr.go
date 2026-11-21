package taint

import (
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// A pointer inside an interface value (Go, ADR-087).
//
// make_interface of a pointer copies what the pointer points to below "/*" of the interface value
// (convert), so a function that is given the interface value, and writes to it (`fmt.Fprint(w, s)`
// with `w io.Writer`, `s.Set(v)` with a pointer receiver), writes to the interface value's cells: the
// copy. What the caller reads is the original (`b.String()`, `b.v`), which the write did not reach.
// ifacePointers finds the pointers an interface value was made of, right there, so that the caller
// applies the write to what they point to as well.

// ifacePointers lists the pointers that the interface value v was made of: through make_interface of a
// pointer, a copy, change_interface and a φ. It is empty when v is not an interface value or its pointer
// is not known here (a parameter: the function's own summary says the write of its parameter).
func ifacePointers(v *sg.Value) []*sg.Value {
	if v == nil || pointerLike(v) {
		return nil
	}
	var out []*sg.Value
	seen := map[*sg.Value]bool{}
	return collectIfacePointers(v, 0, seen, out)
}

func collectIfacePointers(v *sg.Value, depth int, seen map[*sg.Value]bool, out []*sg.Value) []*sg.Value {
	if v == nil || v.Kind != sg.KResult || v.Instr == nil || seen[v] || depth > 8 || len(out) > 16 {
		return out
	}
	seen[v] = true
	in := v.Instr
	switch in.Op {
	case sg.OpPhi, ir.OpCopy:
		for _, a := range in.Args {
			out = collectIfacePointers(a, depth+1, seen, out)
		}
	case ir.OpConvert:
		if in.IR == nil || len(in.Args) != 1 {
			return out
		}
		switch in.IR.Operator {
		case "make_interface":
			if pointerLike(in.Args[0]) {
				out = append(out, in.Args[0])
			}
		case "change_interface":
			out = collectIfacePointers(in.Args[0], depth+1, seen, out)
		}
	}
	return out
}

// ifaceTargets are the cells that a write at path (of the callee's parameter that holds the interface
// value v) is also a write at: the cells that the pointers inside v point to. The path is the one of the
// pointer's target: the "/*" that the interface value keeps it below is left out.
func (s *state) ifaceTargets(v *sg.Value, path string) []Loc {
	ptrs := ifacePointers(v)
	if len(ptrs) == 0 {
		return nil
	}
	if len(path) >= 2 && path[0] == '/' && path[1] == '*' && (len(path) == 2 || path[2] == '/') {
		path = path[2:]
	}
	var out []Loc
	for _, p := range ptrs {
		for _, l := range s.locs(p) {
			out = append(out, Loc{l.Root, l.Path + path})
		}
	}
	return out
}
