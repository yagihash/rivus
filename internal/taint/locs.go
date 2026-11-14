package taint

import (
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"
)

// pointerLike reports whether values of the type refer to storage: the
// location of such a value is the thing it points to, not the value itself.
func pointerLike(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan:
		return true
	}
	return false
}

// resRoot is the root that holds the (first) result value of an instruction.
func resRoot(v ssa.Value) Root {
	switch v := v.(type) {
	case *ssa.TypeAssert:
		if v.CommaOk {
			return Root{v, 0}
		}
	case *ssa.Lookup:
		if v.CommaOk {
			return Root{v, 0}
		}
	case *ssa.UnOp:
		if v.Op == token.ARROW && v.CommaOk {
			return Root{v, 0}
		}
	}
	return own(v)
}

func tupleElem(t types.Type, i int) types.Type {
	if tt, ok := t.(*types.Tuple); ok && i < tt.Len() {
		return tt.At(i).Type()
	}
	return t
}

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
	s.numbering = map[ssa.Value]int{}
	s.locSet = map[ssa.Value]map[Loc]bool{}
	s.locCache = map[ssa.Value][]Loc{}
	n := 0
	for _, p := range s.fn.Params {
		s.numbering[p] = n
		n++
	}
	for _, p := range s.fn.FreeVars {
		s.numbering[p] = n
		n++
	}
	var instrs []ssa.Value
	for _, b := range s.fn.Blocks {
		for _, in := range b.Instrs {
			if v, ok := in.(ssa.Value); ok {
				s.numbering[v] = n
				n++
				instrs = append(instrs, v)
			}
		}
	}
	for {
		changed := false
		for _, v := range instrs {
			for _, l := range s.computeLocs(v) {
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
}

// locs returns the locations value v refers to, in a stable order.
func (s *state) locs(v ssa.Value) []Loc {
	switch v.(type) {
	case *ssa.Const, *ssa.Function, *ssa.Builtin:
		return nil
	case *ssa.Parameter, *ssa.FreeVar:
		return []Loc{{own(v), ""}}
	case *ssa.Global:
		return []Loc{{own(v), ""}}
	}
	if c, ok := s.locCache[v]; ok {
		return c
	}
	set := s.locSet[v]
	out := make([]Loc, 0, len(set))
	for l := range set {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return s.lessLoc(out[i], out[j]) })
	s.locCache[v] = out
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

// computeLocs derives the locations of an instruction's result from the
// current locations of its operands. It is monotone, so iterating it reaches
// a fixed point even through loops.
func (s *state) computeLocs(v ssa.Value) []Loc {
	ownRoot := []Loc{{own(v), ""}}
	switch v := v.(type) {
	case *ssa.FieldAddr:
		return suffix(s.locs(v.X), fieldName(v.X.Type(), v.Field))
	case *ssa.IndexAddr:
		return suffix(s.locs(v.X), "[]")
	case *ssa.Field:
		if pointerLike(v.Type()) {
			return suffix(suffix(s.locs(v.X), fieldName(v.X.Type(), v.Field)), "*")
		}
	case *ssa.Index:
		if pointerLike(v.Type()) {
			return suffix(suffix(s.locs(v.X), "[]"), "*")
		}
	case *ssa.Lookup:
		if !v.CommaOk && pointerLike(v.Type()) {
			return suffix(suffix(s.locs(v.X), "[]"), "*")
		}
	case *ssa.UnOp:
		switch {
		case v.Op == token.MUL && pointerLike(v.Type()):
			return suffix(s.locs(v.X), "*")
		case v.Op == token.ARROW && !v.CommaOk && pointerLike(v.Type()):
			return suffix(suffix(s.locs(v.X), "[]"), "*")
		}
	case *ssa.Phi:
		if pointerLike(v.Type()) {
			var out []Loc
			for _, e := range v.Edges {
				out = append(out, s.locs(e)...)
			}
			return out
		}
	case *ssa.ChangeType:
		if pointerLike(v.Type()) && pointerLike(v.X.Type()) {
			return s.locs(v.X)
		}
	case *ssa.Convert:
		if pointerLike(v.Type()) && pointerLike(v.X.Type()) {
			return s.locs(v.X)
		}
	case *ssa.ChangeInterface:
		return ownRoot
	case *ssa.SliceToArrayPointer:
		return s.locs(v.X)
	case *ssa.Slice:
		if pointerLike(v.Type()) && pointerLike(v.X.Type()) {
			return s.locs(v.X)
		}
	case *ssa.TypeAssert:
		if !v.CommaOk && pointerLike(v.Type()) {
			return suffix(s.locs(v.X), "*")
		}
	case *ssa.Extract:
		return []Loc{{Root{v.Tuple, v.Index}, ""}}
	}
	return ownRoot
}

func fieldName(t types.Type, idx int) string {
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	if st, ok := t.Underlying().(*types.Struct); ok && idx >= 0 && idx < st.NumFields() {
		return st.Field(idx).Name()
	}
	return "?"
}
