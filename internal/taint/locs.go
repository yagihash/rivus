package taint

import (
	"sort"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// pointerLike reports whether the value refers to storage: the location of
// such a value is the thing it points to, not the value itself.
func pointerLike(v *sg.Value) bool { return v != nil && v.IsPointerLike() }

func suffix(locs []Loc, seg string) []Loc {
	out := make([]Loc, len(locs))
	for i, l := range locs {
		out[i] = Loc{l.Root, pathJoin(l.Path, seg)}
	}
	return out
}

// prepareLocs numbers the values of the function and computes, to a fixed
// point, the locations every value refers to.
func (s *state) prepareLocs() {
	s.numbering = map[*sg.Value]int{}
	s.locSet = map[*sg.Value]map[Loc]bool{}
	s.locCache = map[*sg.Value][]Loc{}
	n := 0
	for _, p := range s.fn.Params {
		s.numbering[p] = n
		n++
	}
	for _, p := range s.fn.FreeVars {
		s.numbering[p] = n
		n++
	}
	var vals []*sg.Value
	for _, b := range s.fn.Blocks {
		for _, in := range b.Instrs {
			for _, v := range in.Dsts {
				s.numbering[v] = n
				n++
				vals = append(vals, v)
			}
		}
	}
	// The cache holds the locations once they are final: a value read before
	// the iteration has reached it (a φ of a loop reads what comes round
	// from the end of the body) must not be remembered as having none.
	s.locsFinal = false
	for {
		changed := false
		for _, v := range vals {
			for _, l := range s.computeLocs(v) {
				// a path that grows round a loop (a load of a load of ...)
				// is cut where the cells are: the fixed point must exist
				l.Path, _ = pathTrunc(l.Path, s.eng.opts.K)
				m := s.locSet[v]
				if m == nil {
					m = map[Loc]bool{}
					s.locSet[v] = m
				}
				if !m[l] {
					m[l] = true
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	s.locsFinal = true
}

// locs returns the locations value v refers to, in a stable order.
func (s *state) locs(v *sg.Value) []Loc {
	switch v.Kind {
	case sg.KConst, sg.KFunc, sg.KUndef:
		return nil
	case sg.KParam, sg.KFreeVar, sg.KGlobal:
		return []Loc{{own(v), ""}}
	}
	if c, ok := s.locCache[v]; ok && s.locsFinal {
		return c
	}
	set := s.locSet[v]
	out := make([]Loc, 0, len(set))
	for l := range set {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return s.lessLoc(out[i], out[j]) })
	if s.locsFinal {
		s.locCache[v] = out
	}
	return out
}

func (s *state) lessLoc(a, b Loc) bool {
	na, nb := s.rootNum(a.Root), s.rootNum(b.Root)
	if na != nb {
		return na < nb
	}
	if a.Root.Idx != b.Root.Idx {
		return a.Root.Idx < b.Root.Idx
	}
	// Roots outside the function (package variables) have no number: by name.
	if a.Root.V != nil && b.Root.V != nil && a.Root.V != b.Root.V {
		return a.Root.V.Less(b.Root.V)
	}
	return a.Path < b.Path
}

func (s *state) rootNum(r Root) int {
	if r.V == nil {
		return -1
	}
	if n, ok := s.numbering[r.V]; ok {
		return n
	}
	return 1 << 30 // globals and other roots outside the function
}

// pathSegs spells the access path of an addr_of.
func pathSegs(path []ir.Step) []string {
	var out []string
	for _, st := range path {
		switch st.Kind {
		case ir.StepField:
			out = append(out, st.Field)
		case ir.StepIndex:
			out = append(out, "[]")
		case ir.StepDeref:
			out = append(out, "*")
		}
	}
	return out
}

// Names of the operations a call stands for (see internal/frontend/goir).
const (
	symChanSend   = "chan.send"
	symChanRecv   = "chan.recv"
	symRangeStart = "iter.start"
	symRangeNext  = "iter.next"
	symRangeNextS = "iter.next_string"
	symRunDefers  = "rt.rundefers"
)

// computeLocs derives the locations of an instruction's result from the
// current locations of its operands. It is monotone, so iterating it reaches
// a fixed point even through loops.
func (s *state) computeLocs(v *sg.Value) []Loc {
	ownRoot := []Loc{{own(v), ""}}
	in := v.Instr
	if in == nil {
		return ownRoot
	}
	if in.Op == sg.OpPhi {
		if pointerLike(v) {
			var out []Loc
			for _, e := range in.Args {
				out = append(out, s.locs(e)...)
			}
			return out
		}
		return ownRoot
	}
	if len(in.Dsts) == 0 || in.Dsts[0] != v {
		return ownRoot // a result after the first: its own root
	}
	switch in.Op {
	case ir.OpAddrOf:
		if in.IR.Global != "" || len(in.Args) == 0 {
			return ownRoot
		}
		out := s.locs(in.Args[0])
		for _, seg := range pathSegs(in.IR.Path) {
			out = suffix(out, seg)
		}
		return out
	case ir.OpFieldLoad:
		if pointerLike(v) {
			return suffix(suffix(s.locs(in.Args[0]), in.IR.Field), "*")
		}
	case ir.OpIndexLoad:
		if len(in.Dsts) == 1 && pointerLike(v) {
			return suffix(suffix(s.locs(in.Args[0]), "[]"), "*")
		}
	case ir.OpLoad, ir.OpGlobalLoad:
		if pointerLike(v) {
			return suffix(s.loadAddr(in), "*")
		}
	case ir.OpConvert:
		switch in.IR.Operator {
		case "change_interface", "make_interface":
		case "slice_to_array_pointer":
			return s.locs(in.Args[0])
		default:
			if pointerLike(v) && pointerLike(in.Args[0]) {
				return s.locs(in.Args[0])
			}
		}
	case ir.OpCopy:
		// A copy of a pointer is the same pointer: what is written through
		// one is read through the other.
		if pointerLike(v) && pointerLike(in.Args[0]) {
			return s.locs(in.Args[0])
		}
	case ir.OpTypeAssert:
		if len(in.Dsts) == 1 && pointerLike(v) {
			return suffix(s.locs(in.Args[0]), "*")
		}
	case ir.OpCall:
		if in.Callee.Symbol == symChanRecv && len(in.Dsts) == 1 && pointerLike(v) {
			return suffix(suffix(s.locs(in.Args[0]), "[]"), "*")
		}
	}
	return ownRoot
}

// loadAddr is where a load reads: the locations of its address operand, or
// of the package variable.
func (s *state) loadAddr(in *sg.Instr) []Loc {
	if in.Op == ir.OpGlobalLoad {
		return s.locs(s.eng.prog.Global(in.IR.Global))
	}
	return s.locs(in.Args[0])
}
