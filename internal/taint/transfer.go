package taint

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/yagihash/rivus/internal/sites"
)

// analyze runs the function-local analysis to a fixed point.
func (s *state) analyze() {
	s.prepareLocs()
	s.seed()
	for iter := 0; iter < 50; iter++ {
		s.changed = false
		for _, b := range s.fn.Blocks {
			for _, in := range b.Instrs {
				if s.adds > s.eng.opts.Budget {
					s.exceeded = true
					return
				}
				s.transfer(in)
			}
		}
		if !s.changed {
			return
		}
	}
	s.eng.noteNotConverged(s.fn)
}

// seed gives every parameter its symbolic taint, and the real taint of
// param_type sources.
func (s *state) seed() {
	idx := 0
	for _, p := range s.fn.Params {
		s.seedParam(p, idx)
		idx++
	}
	for _, p := range s.fn.FreeVars {
		s.seedParam(p, idx)
		idx++
	}
}

func (s *state) seedParam(v ssa.Value, idx int) {
	r := own(v)
	s.paramRoot[r] = idx
	s.add(Loc{r, ""}, Elem{Kind: Param, Param: idx}, s.origin(v.Pos(), "param"))
	if p, ok := v.(*ssa.Parameter); ok {
		if site := s.eng.sites.ByParam[p]; site != nil {
			for _, l := range site.Labels {
				s.add(Loc{r, ""}, Elem{Kind: Real, Label: l, Origin: site.ID}, s.origin(p.Pos(), "source: parameter "+site.Symbol))
			}
		}
	}
}

func isComparison(op token.Token) bool {
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	}
	return false
}

func (s *state) transfer(in ssa.Instruction) {
	switch in := in.(type) {
	case *ssa.Phi:
		if pointerLike(in.Type()) {
			return
		}
		for _, e := range in.Edges {
			s.copyLocs(s.locs(e), []Loc{{own(in), ""}}, "phi", in)
		}

	case *ssa.UnOp:
		switch in.Op {
		case token.MUL:
			if pointerLike(in.Type()) {
				return
			}
			src := s.locs(in.X)
			s.copyLocsNoted(src, []Loc{{own(in), ""}}, "load", in, s.globalNote(src, in.Pos()))
		case token.ARROW:
			s.loadInto(in, Loc{resRoot(in), ""}, suffix(s.locs(in.X), "[]"), tupleElem(in.Type(), 0), "receive", s.channelNote(in.Pos()))
		default:
			s.put(Loc{own(in), ""}, s.flatten(s.locs(in.X)), s.whyFn(in, "unary operation"))
		}

	case *ssa.BinOp:
		if isComparison(in.Op) {
			return
		}
		fs := append(s.flatten(s.locs(in.X)), s.flatten(s.locs(in.Y))...)
		s.put(Loc{own(in), ""}, fs, s.whyFn(in, "operation"))

	case *ssa.ChangeType:
		s.convert(in, in.X)
	case *ssa.Convert:
		s.convert(in, in.X)
	case *ssa.ChangeInterface:
		s.copyLocs(s.locs(in.X), []Loc{{own(in), ""}}, "convert", in)
	case *ssa.SliceToArrayPointer:
		// aliases the slice

	case *ssa.MakeInterface:
		dst := Loc{own(in), ""}
		if pointerLike(in.X.Type()) {
			dst.Path = "/*"
		}
		s.copyLocs(s.locs(in.X), []Loc{dst}, "convert", in)

	case *ssa.TypeAssert:
		if !in.CommaOk && pointerLike(in.Type()) {
			return
		}
		s.loadInto(in, Loc{resRoot(in), ""}, s.locs(in.X), tupleElem(in.Type(), 0), "type assertion", nil)

	case *ssa.Slice:
		if pointerLike(in.Type()) && pointerLike(in.X.Type()) {
			return
		}
		s.copyLocs(s.locs(in.X), []Loc{{own(in), ""}}, "slice", in)

	case *ssa.Field:
		s.fieldSources(in, s.locs(in.X), fieldName(in.X.Type(), in.Field))
		if pointerLike(in.Type()) {
			return
		}
		s.copyLocs(suffix(s.locs(in.X), fieldName(in.X.Type(), in.Field)), []Loc{{own(in), ""}}, "read field", in)

	case *ssa.FieldAddr:
		s.fieldSources(in, s.locs(in.X), fieldName(in.X.Type(), in.Field))

	case *ssa.Index:
		if pointerLike(in.Type()) {
			return
		}
		s.copyLocs(suffix(s.locs(in.X), "[]"), []Loc{{own(in), ""}}, "read element", in)

	case *ssa.Lookup:
		s.loadInto(in, Loc{resRoot(in), ""}, suffix(s.locs(in.X), "[]"), tupleElem(in.Type(), 0), "read element", nil)

	case *ssa.Range:
		s.copyLocs(s.locs(in.X), []Loc{{own(in), ""}}, "range", in)

	case *ssa.Next:
		iter := s.locs(in.Iter)
		s.copyLocs(suffix(iter, "[k]"), []Loc{{Root{in, 1}, ""}}, "range key", in)
		if in.IsString {
			s.copyLocs(iter, []Loc{{Root{in, 2}, ""}}, "range value", in)
		} else {
			s.loadInto(in, Loc{Root{in, 2}, ""}, suffix(iter, "[]"), tupleElem(in.Type(), 2), "range value", nil)
		}

	case *ssa.Store:
		s.store(in, s.locs(in.Addr), in.Val, "", nil)
	case *ssa.MapUpdate:
		m := s.locs(in.Map)
		for _, l := range m {
			s.put(Loc{l.Root, pathJoin(l.Path, "[k]")}, s.flatten(s.locs(in.Key)), s.whyFn(in, "map key"))
		}
		s.store(in, m, in.Value, "[]", nil)
	case *ssa.Send:
		s.store(in, s.locs(in.Chan), in.X, "[]", s.channelNote(in.Pos()))
	case *ssa.Select:
		note := s.channelNote(in.Pos())
		k := 0
		for _, st := range in.States {
			if st.Dir == types.SendOnly {
				s.store(in, s.locs(st.Chan), st.Send, "[]", note)
				continue
			}
			s.loadInto(in, Loc{Root{in, 2 + k}, ""}, suffix(s.locs(st.Chan), "[]"), tupleElem(in.Type(), 2+k), "receive", note)
			k++
		}

	case *ssa.Return:
		for i, r := range in.Results {
			s.copyLocs(s.locs(r), []Loc{{Root{nil, i}, ""}}, "return", in)
		}

	case ssa.CallInstruction:
		s.call(in)
	}
}

// loadInto copies the value stored at src into dst, for results that are
// distinct roots (tuples). Pointer-like values are read through the pointer.
func (s *state) loadInto(in ssa.Instruction, dst Loc, src []Loc, t types.Type, kind string, note *Uncertainty) {
	if pointerLike(t) {
		src = suffix(src, "*")
	}
	s.copyLocsNoted(src, []Loc{dst}, kind, in, note)
}

// channelNote marks a hop through a channel: goroutines are not modelled, so
// the order in which values arrive is unknown.
func (s *state) channelNote(pos token.Pos) *Uncertainty {
	return s.pathNote(KindConcurrency, "the value crosses a channel; the order of effects between goroutines is not modelled", pos)
}

// globalNote marks a read of a package variable, listing who writes it.
func (s *state) globalNote(src []Loc, pos token.Pos) *Uncertainty {
	for _, l := range src {
		if g, ok := l.Root.V.(*ssa.Global); ok {
			u := s.pathNote(KindGlobalVar, "the value is read from the package variable "+g.String(), pos)
			u.Symbol = g.String()
			u.Candidates = s.eng.writersOf(g)
			return u
		}
	}
	return nil
}

func (s *state) convert(in ssa.Value, x ssa.Value) {
	switch {
	case pointerLike(in.Type()) && pointerLike(x.Type()):
		// alias
	case pointerLike(in.Type()):
		// e.g. string to []byte: the content becomes the elements
		s.put(Loc{own(in), pathJoin("", "[]")}, s.flatten(s.locs(x)), s.whyFn(in.(ssa.Instruction), "convert"))
	default:
		s.copyLocs(s.locs(x), []Loc{{own(in), ""}}, "convert", in.(ssa.Instruction))
	}
}

// store writes val to the cells addr refers to (below the extra segment).
func (s *state) store(in ssa.Instruction, addr []Loc, val ssa.Value, seg string, note *Uncertainty) {
	var dst []Loc
	for _, l := range addr {
		p := l.Path
		if seg != "" {
			p = pathJoin(p, seg)
		}
		if pointerLike(val.Type()) {
			p = pathJoin(p, "*")
		}
		dst = append(dst, Loc{l.Root, p})
	}
	s.copyLocsNoted(s.locs(val), dst, "store", in, note)
}

// fieldSources applies field rules: reading a field that is declared a
// source introduces taint.
func (s *state) fieldSources(in ssa.Instruction, base []Loc, field string) {
	for _, site := range s.eng.sites.ByInstr[in] {
		if site.Role != sites.RoleSource {
			continue
		}
		for _, l := range base {
			for _, label := range site.Labels {
				s.add(Loc{l.Root, pathJoin(l.Path, field)}, Elem{Kind: Real, Label: label, Origin: site.ID},
					s.origin(in.Pos(), "source: field "+site.Symbol))
			}
		}
	}
}
