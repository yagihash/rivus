package taint

import (
	"sort"
	"time"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// Pointers held in cells (ENV-PTR).
//
// A store of a pointer into a cell writes what the pointer points to into the cell's "/*" (store): the
// cell holds a copy of the content, and what is later written through the pointer that is loaded from
// the cell lands in that copy, not in the variable. A closure of Rust that captures a variable by `&mut`
// holds a pointer to it in a field of its environment, and writes through it: the variable never saw
// the write.
//
// So that it does, the cell remembers where the pointer pointed (ptrAlias: the slot -> the locations of
// the pointee), and
//   * the locations of a pointer loaded from the slot are the pointee too (pointees), so a write through
//     it, in this function, goes to the variable as well;
//   * what a callee writes at "<slot>/*..." (a closure called with its environment, a helper given the
//     struct) is written at the pointee as well (mirror, used by applySummary).
//
// The slot's copy stays: the reads of the content still find what was written to the variable (the
// stores are applied again as the variable changes). Only for a program that has closure environments
// (sg.Program.HasEnv: Rust). A struct that is stored whole into another cell (the environment into the
// variable of the closure) takes its slots with it, and a write through a pointer in the copy is a write
// through the pointer in the original (copyOf: `mut f: impl FnMut()` is copied into a slot of the helper,
// and the closure that the helper calls writes into the slot).

// collectPtrStores lists the stores that make ptrAlias. Nothing for a program that has no closure
// environment.
func (s *state) collectPtrStores() {
	s.ptrStores, s.valStores, s.ptrAlias, s.copyOf = s.ptrStores[:0], s.valStores[:0], nil, nil
	hasEnv := s.eng.prog.HasEnv
	for _, b := range s.fn.Blocks {
		for _, in := range b.Instrs {
			var val *sg.Value
			switch in.Op {
			case ir.OpStore:
				val = in.Args[1]
				if !hasEnv && !(pointerLike(val) && isLocalCell(in.Args[0])) {
					continue
				}
			case ir.OpFieldStore:
				val = in.Args[1]
				if !hasEnv {
					continue
				}
			default:
				continue
			}
			if pointerLike(val) {
				s.ptrStores = append(s.ptrStores, in)
			} else if holdsPointers(val) {
				s.valStores = append(s.valStores, in)
			}
		}
	}
}

// isLocalCell reports whether the address is a variable of the function that the closures capture (an Alloc). Go has no
// closure environment (HasEnv is false), but it keeps a variable that a closure captures in such a cell, and a parameter
// that is captured is stored into one (`o` of `func (o *T) f()`): without this, what is written through the pointer that
// is loaded from the cell is written to the copy in the cell, and the summary does not say that the parameter was written
// (no finding, and no uncertainty).
func isLocalCell(addr *sg.Value) bool {
	return addr != nil && addr.Kind == sg.KResult && addr.Instr != nil && addr.Instr.Op == ir.OpAlloc
}

// holdsPointers reports whether a value that is not a pointer can hold one: a struct, a closure's
// environment, a function value, an interface, or a type that the extractor has no class for (a generic
// parameter: `mut f: impl FnMut()`). A number, a bool, a string and a constant cannot.
func holdsPointers(v *sg.Value) bool {
	if v.Kind == sg.KConst || v.Kind == sg.KUndef || v.Kind == sg.KFunc {
		return false
	}
	switch v.Class {
	case ir.ClassStruct, ir.ClassFunc, ir.ClassOther, ir.ClassInterface:
		return true
	}
	return false
}

// slotsOf are the cells a store writes into.
func (s *state) slotsOf(in *sg.Instr) []Loc {
	if in.Op == ir.OpFieldStore {
		return suffix(s.locs(in.Args[0]), in.IR.Field)
	}
	return s.locs(in.Args[0])
}

func (s *state) addAlias(slot, target Loc) bool {
	if slot == target {
		return false
	}
	m := s.ptrAlias[slot]
	if m == nil {
		if s.ptrAlias == nil {
			s.ptrAlias = map[Loc]map[Loc]struct{}{}
		}
		m = map[Loc]struct{}{}
		s.ptrAlias[slot] = m
	}
	if _, ok := m[target]; ok {
		return false
	}
	m[target] = struct{}{}
	return true
}

func (s *state) addCopy(dst, src Loc) bool {
	if dst == src {
		return false
	}
	for _, x := range s.copyOf[dst] {
		if x == src {
			return false
		}
	}
	if s.copyOf == nil {
		s.copyOf = map[Loc][]Loc{}
	}
	s.copyOf[dst] = append(s.copyOf[dst], src)
	return true
}

// aliasCheckEvery is how many steps of the loops of buildPtrAlias pass between two looks at the clock.
const aliasCheckEvery = 4096

// buildPtrAlias adds what the stores say to ptrAlias, from the locations known now. It reports whether
// something was added (prepareLocs goes on until nothing is), and whether it was done before limit: the
// loops count their steps and look at the clock now and then, like the rest of the function's analysis
// (-timeout, -func-timeout).
func (s *state) buildPtrAlias(limit time.Time) (changed, inTime bool) {
	if len(s.ptrStores) == 0 && len(s.valStores) == 0 {
		return false, true
	}
	steps := 0
	for _, in := range s.valStores {
		// a value that is not a pointer is stored whole (a struct, a closure's environment): what is
		// written through a pointer inside the copy is written through the pointer inside the original
		srcs := s.locs(in.Args[1])
		if len(srcs) == 0 {
			continue
		}
		for _, d := range s.slotsOf(in) {
			for _, src := range srcs {
				if steps++; steps%aliasCheckEvery == 0 && time.Now().After(limit) {
					return changed, false
				}
				if s.addCopy(d, src) {
					changed = true
				}
			}
		}
	}
	for _, in := range s.ptrStores {
		targets := s.locs(in.Args[1])
		if len(targets) == 0 {
			continue
		}
		for _, slot := range s.slotsOf(in) {
			for _, t := range targets {
				if steps++; steps%aliasCheckEvery == 0 && time.Now().After(limit) {
					return changed, false
				}
				if s.addAlias(slot, t) {
					changed = true
				}
			}
		}
	}
	// a struct stored whole into a cell: the slots inside it are inside the copy
	for len(s.ptrAlias) > 0 {
		grew := false
		for _, in := range s.valStores {
			srcs := s.locs(in.Args[1])
			if len(srcs) == 0 {
				continue
			}
			var dsts []Loc
			for _, src := range srcs {
				for slot, ts := range s.ptrAlias {
					if steps++; steps%aliasCheckEvery == 0 && time.Now().After(limit) {
						return changed, false
					}
					if slot.Root != src.Root || !pathHasPrefix(slot.Path, src.Path) {
						continue
					}
					if dsts == nil {
						dsts = s.slotsOf(in)
					}
					for _, d := range dsts {
						nk := Loc{d.Root, d.Path + slot.Path[len(src.Path):]}
						for t := range ts {
							if steps++; steps%aliasCheckEvery == 0 && time.Now().After(limit) {
								return changed, false
							}
							if s.addAlias(nk, t) {
								grew = true
							}
						}
					}
				}
			}
		}
		if !grew {
			break
		}
		changed = true
	}
	return changed, true
}

// pointees are the locations that the pointers held in these cells point to.
func (s *state) pointees(slots []Loc) []Loc {
	if len(s.ptrAlias) == 0 {
		return nil
	}
	var out []Loc
	for _, sl := range slots {
		for t := range s.ptrAlias[sl] {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return s.lessLoc(out[i], out[j]) })
	return out
}

// mirror are the cells that a write at l is also a write at: the pointee of a pointer that is held on the
// way (l is "<slot>/*<rest>", and the pointer in <slot> points to t: then t<rest>).
func (s *state) mirror(l Loc) []Loc {
	if len(s.ptrAlias) == 0 && len(s.copyOf) == 0 {
		return nil
	}
	var out []Loc
	seen := map[Loc]bool{l: true}
	work := []Loc{l}
	for len(work) > 0 && len(seen) < 64 {
		c := work[len(work)-1]
		work = work[:len(work)-1]
		p := c.Path
		for i := 0; i+2 <= len(p); i++ {
			if p[i] != '/' || p[i+1] != '*' || (i+2 < len(p) && p[i+2] != '/') {
				continue
			}
			for t := range s.ptrAlias[Loc{c.Root, p[:i]}] {
				nl := Loc{t.Root, t.Path + p[i+2:]}
				if !seen[nl] {
					seen[nl] = true
					out = append(out, nl)
					work = append(work, nl)
				}
			}
		}
		// a struct that was copied: a write through a pointer in the copy is a write through the pointer in
		// the original
		if len(s.copyOf) > 0 {
			for i := 0; i <= len(p); i++ {
				if i < len(p) && p[i] != '/' {
					continue
				}
				rest := p[i:]
				if !throughPointer(rest) {
					continue
				}
				for _, src := range s.copyOf[Loc{c.Root, p[:i]}] {
					nl := Loc{src.Root, src.Path + rest}
					if !seen[nl] {
						seen[nl] = true
						out = append(out, nl)
						work = append(work, nl)
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return s.lessLoc(out[i], out[j]) })
	return out
}

// throughPointer reports whether the path has a step that follows a pointer ("/*" as a whole step).
func throughPointer(p string) bool {
	for i := 0; i+2 <= len(p); i++ {
		if p[i] == '/' && p[i+1] == '*' && (i+2 == len(p) || p[i+2] == '/') {
			return true
		}
	}
	return false
}

// pointerAliasDetail is the text of the note on a hop that went through the approximation of a write via a
// pointer (ptrAlias, copyOf).
const pointerAliasDetail = "this step goes through a write or a read via a pointer that rivus approximates by aliasing: the " +
	"pointer held in a cell is taken to point to the places it was stored from, whichever of them it points to at run time. " +
	"The flow may not exist; read the step and check that the pointer really points to that variable here"

func (s *state) aliasNote(pos ir.Pos) *Uncertainty {
	return s.pathNote(KindPointerAlias, pointerAliasDetail, pos)
}

// ptrAliasLocs are the locations of the pointer v that are there only because of ptrAlias (the pointee of a
// pointer loaded from a cell): the ones that a pointer the cell does not have would not reach. Nothing when
// the function has no ptrAlias.
func (s *state) ptrAliasLocs(v *sg.Value) map[Loc]bool {
	if len(s.ptrAlias) == 0 || v == nil {
		return nil
	}
	alias, direct := map[Loc]bool{}, map[Loc]bool{}
	s.splitAlias(v, alias, direct, map[*sg.Value]bool{})
	for l := range direct {
		delete(alias, l)
	}
	if len(alias) == 0 {
		return nil
	}
	return alias
}

func (s *state) splitAlias(v *sg.Value, alias, direct map[Loc]bool, seen map[*sg.Value]bool) {
	add := func(m map[Loc]bool, ls []Loc) {
		for _, l := range ls {
			l.Path, _ = pathTrunc(l.Path, s.eng.opts.K)
			m[l] = true
		}
	}
	in := v.Instr
	if in == nil || seen[v] || len(in.Dsts) == 0 || in.Dsts[0] != v {
		add(direct, s.locs(v))
		return
	}
	seen[v] = true
	switch {
	case in.Op == sg.OpPhi && pointerLike(v):
		for _, e := range in.Args {
			s.splitAlias(e, alias, direct, seen)
		}
	case in.Op == ir.OpFieldLoad && pointerLike(v):
		slots := suffix(s.locs(in.Args[0]), in.IR.Field)
		add(direct, suffix(slots, "*"))
		add(alias, s.pointees(slots))
	case (in.Op == ir.OpLoad || in.Op == ir.OpGlobalLoad) && pointerLike(v):
		slots := s.loadAddr(in)
		add(direct, suffix(slots, "*"))
		add(alias, s.pointees(slots))
	case in.Op == ir.OpCopy && pointerLike(v) && pointerLike(in.Args[0]):
		s.splitAlias(in.Args[0], alias, direct, seen)
	case in.Op == ir.OpConvert && pointerLike(v) && pointerLike(in.Args[0]) &&
		in.IR.Operator != "change_interface" && in.IR.Operator != "make_interface":
		s.splitAlias(in.Args[0], alias, direct, seen)
	default:
		add(direct, s.locs(v))
	}
}

// markAliased returns the locations of ls that are in the set, and the ones that are not.
func markAliased(ls []Loc, set map[Loc]bool) (via, plain []Loc) {
	for _, l := range ls {
		if set[l] {
			via = append(via, l)
		} else {
			plain = append(plain, l)
		}
	}
	return via, plain
}

// copyLocsAlias is copyLocsNoted for a copy where some of the source or of the destination locations are
// there only because of ptrAlias (srcAlias, dstAlias): the hops that read or write them carry the
// pointer_alias note.
func (s *state) copyLocsAlias(src, dst []Loc, srcAlias, dstAlias map[Loc]bool, kind string, in *sg.Instr, note *Uncertainty) {
	if len(srcAlias) == 0 && len(dstAlias) == 0 {
		s.copyLocsNoted(src, dst, kind, in, note)
		return
	}
	srcVia, srcPlain := markAliased(src, srcAlias)
	dstVia, dstPlain := markAliased(dst, dstAlias)
	s.copyLocsNoted(srcPlain, dstPlain, kind, in, note)
	an := s.aliasNote(in.Pos)
	if note == nil {
		s.copyLocsNoted(srcVia, dstPlain, kind, in, an)
		s.copyLocsNoted(srcPlain, dstVia, kind, in, an)
		s.copyLocsNoted(srcVia, dstVia, kind, in, an)
		return
	}
	for _, p := range [][2][]Loc{{srcVia, dstPlain}, {srcPlain, dstVia}, {srcVia, dstVia}} {
		if len(p[0]) == 0 || len(p[1]) == 0 {
			continue
		}
		if len(s.opaque) > 0 {
			s.carryOpaque(p[0], p[1])
		}
		base := s.whyFn(in, kind)
		for _, d := range p[1] {
			for _, sl := range p[0] {
				s.copyLoc(sl, d, func(f found) *why {
					w := base(f)
					w.setNotes(appendNote(appendNote(w.notes(), note), an))
					return w
				})
			}
		}
	}
}

// withAliasNote is the hop with the pointer_alias note added (a copy: the hop is shared with the other
// destinations of the write).
func (s *state) withAliasNote(w *why, pos ir.Pos) *why {
	c := *w
	c.rare = w.copyRare()
	c.setNotes(appendNote(c.notes(), s.aliasNote(pos)))
	return &c
}
