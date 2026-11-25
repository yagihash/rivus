package taint

import (
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
	"time"
)

// analyze runs the function-local analysis to a fixed point.
func (s *state) analyze() {
	limit := time.Now().Add(s.eng.opts.FuncTimeout)
	timeLimit := degFuncTimeout
	if d := s.eng.deadline; !d.IsZero() && d.Before(limit) {
		limit = d
		timeLimit = degRunTimeout
	}
	if !s.prepareLocs(limit) {
		s.exceeded, s.exceededBy = true, timeLimit
		return
	}
	s.seed()
	steps := 0
	for iter := 0; iter < 50; iter++ {
		s.changed = false
		for _, b := range s.fn.Blocks {
			for _, in := range b.Instrs {
				steps++
				if s.adds > s.eng.opts.Budget {
					s.exceeded, s.exceededBy = true, degBudget
					return
				}
				if steps%64 == 0 && time.Now().After(limit) {
					s.exceeded, s.exceededBy = true, timeLimit
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

func (s *state) seedParam(v *sg.Value, idx int) {
	r := own(v)
	s.paramRoot[r] = idx
	seed := ParamElem(idx, "")
	if s.eng.opts.CharTaint && s.numericRoot(r) {
		seed = seed.WithChar(true) // a number that a character may be passed as: what the callee does with it is followed
	}
	s.add(Loc{r, ""}, seed, s.origin(v.Pos, "param"))
	if v.Kind == sg.KParam {
		for _, site := range s.eng.sites.ByIRParam[sites.IRParam{Func: s.fn.IR.ID, Index: idx}] {
			for _, l := range site.Labels {
				s.add(Loc{r, ""}, RealElem(l, site.ID), s.origin(v.Pos, "source: parameter "+site.Symbol))
			}
		}
	}
}

func isComparison(op string) bool {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=", "is", "is!":
		return true
	}
	return false
}

func (s *state) transfer(in *sg.Instr) {
	switch in.Op {
	case sg.OpPhi:
		dst := in.Dsts[0]
		if pointerLike(dst) {
			return
		}
		preds := in.Block.Preds
		for i, e := range in.Args {
			var gs []Guard
			if i < len(preds) {
				gs = s.edgeGuards(preds[i], in.Block)
			}
			s.copyLocsGuarded(s.locs(e), []Loc{{own(dst), ""}}, "phi", in, gs)
		}

	case ir.OpLoad, ir.OpGlobalLoad:
		dst := in.Dsts[0]
		if pointerLike(dst) {
			return
		}
		src := s.loadAddr(in)
		var al map[Loc]bool
		if in.Op == ir.OpLoad {
			al = s.ptrAliasLocs(in.Args[0])
		}
		s.copyLocsAlias(src, []Loc{{own(dst), ""}}, al, nil, "load", in, s.globalNote(src, in.Pos))

	case ir.OpBinop:
		if isComparison(in.IR.Operator) {
			return
		}
		fs := append(s.flatten(s.locs(in.Args[0])), s.flatten(s.locs(in.Args[1]))...)
		s.put(Loc{own(in.Dsts[0]), ""}, fs, s.whyFn(in, "operation"))

	case ir.OpUnop:
		s.put(Loc{own(in.Dsts[0]), ""}, s.flatten(s.locs(in.Args[0])), s.whyFn(in, "unary operation"))

	case ir.OpConvert:
		s.convert(in)

	case ir.OpTypeAssert:
		dst := in.Dsts[0]
		if len(in.Dsts) == 1 && pointerLike(dst) {
			return
		}
		s.loadInto(in, Loc{own(dst), ""}, s.locs(in.Args[0]), pointerLike(dst), "type assertion", nil)

	case ir.OpCopy:
		dst, x := in.Dsts[0], in.Args[0]
		if in.IR.View {
			if pointerLike(dst) && pointerLike(x) {
				return
			}
			s.copyLocs(s.locs(x), []Loc{{own(dst), ""}}, "slice", in)
			return
		}
		if pointerLike(dst) && pointerLike(x) {
			return // the same pointer
		}
		s.copyLocs(s.locs(x), []Loc{{own(dst), ""}}, "copy", in)

	case ir.OpFieldLoad:
		dst := in.Dsts[0]
		base := s.locs(in.Args[0])
		field := in.IR.Field
		s.fieldSources(in, base, field)
		if pointerLike(dst) {
			return
		}
		src := suffix(base, field)
		al := s.aliasSuffix(in.Args[0], field)
		if in.IR.Folded {
			s.copyLocsAlias(src, []Loc{{own(dst), ""}}, al, nil, "load", in, s.globalNote(src, in.Pos))
		} else {
			s.copyLocsAlias(src, []Loc{{own(dst), ""}}, al, nil, "read field", in, nil)
		}

	case ir.OpFieldStore:
		base := s.locs(in.Args[0])
		field := in.IR.Field
		s.fieldSources(in, base, field)
		s.storeAlias(in, suffix(base, field), in.Args[1], s.aliasSuffix(in.Args[0], field))

	case ir.OpAddrOf:
		if in.IR.Global != "" || len(in.IR.Path) == 0 || in.IR.Path[0].Kind != ir.StepField {
			return
		}
		s.fieldSources(in, s.locs(in.Args[0]), in.IR.Path[0].Field)

	case ir.OpIndexLoad:
		dst := in.Dsts[0]
		src := suffix(s.locs(in.Args[0]), "[]")
		if s.eng.opts.CharTaint {
			s.tableLookup(in)
		}
		if in.IR.CharRead && s.eng.opts.CharTaint {
			s.markChar = true // what the extractor says (ir.Instr.CharRead): the value is one character of the text
			defer func() { s.markChar = false }()
		}
		switch {
		case in.IR.Keyed:
			s.loadInto(in, Loc{own(dst), ""}, src, pointerLike(dst), "read element", s.globalNote(src, in.Pos))
		case in.IR.Folded:
			if pointerLike(dst) {
				return
			}
			s.copyLocsNoted(src, []Loc{{own(dst), ""}}, "load", in, s.globalNote(src, in.Pos))
		default:
			if pointerLike(dst) {
				return
			}
			s.copyLocs(src, []Loc{{own(dst), ""}}, "read element", in)
		}

	case ir.OpIndexStore:
		base := s.locs(in.Args[0])
		if in.IR.Keyed {
			for _, l := range base {
				s.put(Loc{l.Root, pathJoin(l.Path, "[k]")}, s.flatten(s.locs(in.Args[1])), s.whyFn(in, "map key"))
			}
			s.store(in, base, in.Args[2], "[]", nil)
			return
		}
		s.store(in, suffix(base, "[]"), in.Args[2], "", nil)

	case ir.OpStore:
		s.storeAlias(in, s.locs(in.Args[0]), in.Args[1], s.ptrAliasLocs(in.Args[0]))
	case ir.OpGlobalStore:
		s.store(in, s.locs(s.eng.prog.Global(in.IR.Global)), in.Args[0], "", nil)

	case sg.OpReturn:
		gs := s.blockGuards(in.Block)
		for i, r := range in.Args {
			s.copyLocsGuarded(s.locs(r), []Loc{{Root{nil, i}, ""}}, "return", in, gs)
		}

	case sg.OpThrow:
		if !s.eng.prog.HasExceptions {
			return
		}
		gs := s.blockGuards(in.Block)
		for _, v := range in.Args {
			// the thrown value is boxed, as in an interface: an object is what the
			// pointer stored here points to
			d := Loc{excRoot(), ""}
			if pointerLike(v) {
				d.Path = "/*"
			}
			s.copyLocsGuarded(s.locs(v), []Loc{d}, "throw", in, gs)
		}

	case ir.OpExceptionValue:
		// the value a handler receives: what the function throws, and what its
		// calls throw (the thrown values are not told apart)
		if len(in.Dsts) > 0 && s.eng.prog.HasExceptions {
			s.copyLocs([]Loc{{excRoot(), ""}}, []Loc{{own(in.Dsts[0]), ""}}, "caught", in)
			// A thrown object is boxed (`/*`), as in an interface. The handler either takes it out
			// with a cast (`e as T`, which strips the box: the first copy is what it reads), or uses the
			// variable as it is (`on T catch (e)`, a dynamic `e.field`): then the object is the value
			// itself, and the second copy gives it that. The value has both shapes, which says more than
			// is true, not less.
			s.copyLocs([]Loc{{excRoot(), "/*"}}, []Loc{{own(in.Dsts[0]), ""}}, "caught", in)
		}

	case ir.OpUnsupported:
		// what the extractor could not express is not in the analysis: say so, where it is
		reason := ""
		if in.IR != nil {
			reason = in.IR.Reason
		}
		note := s.newNote(in, KindUnsupported, "the extractor could not express a construct here ("+reason+"): what it does is not followed", "")
		// a construct that makes a value says what it was made from: the result comes from those, more than the
		// truth and never less, and the note is on the way (a finding that goes through it is not a clear one)
		if len(in.Dsts) > 0 && len(in.Args) > 0 {
			s.put(Loc{own(in.Dsts[0]), ""}, s.flatten(valuesLocs(s, in.Args)), s.whyNote(in, "unsupported: "+reason, note))
		}

	case ir.OpCall:
		s.callInstr(in)

	case ir.OpMakeClosure:
		if s.funcCellsOn() && len(in.Dsts) > 0 && in.IR != nil && in.IR.Func != "" {
			s.seedFunc(Loc{own(in.Dsts[0]), ""}, string(in.IR.Func), in.Pos, "function value: closure")
		}
		s.captured(in)
	}
}

// callInstr handles the calls that stand for operations of the language
// (channels, iteration); the others are calls.
func (s *state) callInstr(in *sg.Instr) {
	if !reservedShapeOK(in) {
		// ir.Validate refuses this; a program that did not pass through it is
		// handled as a plain call rather than indexed into.
		s.call(in)
		return
	}
	switch in.Callee.Symbol {
	case symChanSend:
		s.store(in, s.locs(in.Args[0]), in.Args[1], "[]", s.channelNote(in.Pos))
	case symChanRecv:
		dst := in.Dsts[0]
		s.loadInto(in, Loc{own(dst), ""}, suffix(s.locs(in.Args[0]), "[]"), pointerLike(dst), "receive", s.channelNote(in.Pos))
	case symRangeStart:
		s.copyLocs(s.locs(in.Args[0]), []Loc{{own(in.Dsts[0]), ""}}, "range", in)
	case symRangeNext, symRangeNextS:
		iter := s.locs(in.Args[0])
		s.copyLocs(suffix(iter, "[k]"), []Loc{{own(in.Dsts[1]), ""}}, "range key", in)
		if in.Callee.Symbol == symRangeNextS {
			s.markChar = in.IR != nil && in.IR.CharRead && s.eng.opts.CharTaint // the value is one character, the key is not
			s.copyLocs(iter, []Loc{{own(in.Dsts[2]), ""}}, "range value", in)
			s.markChar = false
		} else {
			s.loadInto(in, Loc{own(in.Dsts[2]), ""}, suffix(iter, "[]"), pointerLike(in.Dsts[2]), "range value", nil)
		}
	case symRunDefers:
	default:
		s.call(in)
	}
}

// loadInto copies the value stored at src into dst. Pointer-like values are
// read through the pointer.
func (s *state) loadInto(in *sg.Instr, dst Loc, src []Loc, ptr bool, kind string, note *Uncertainty) {
	if ptr {
		src = suffix(src, "*")
	}
	s.copyLocsNoted(src, []Loc{dst}, kind, in, note)
}

// channelNote marks a hop through a channel: goroutines are not modelled, so
// the order in which values arrive is unknown.
func (s *state) channelNote(pos ir.Pos) *Uncertainty {
	return s.pathNote(KindConcurrency, "the value crosses a channel; the order of effects between goroutines is not modelled", pos)
}

// globalNote marks a read of a package variable, listing who writes it.
func (s *state) globalNote(src []Loc, pos ir.Pos) *Uncertainty {
	for _, l := range src {
		if l.Root.V != nil && l.Root.V.Kind == sg.KGlobal {
			g := l.Root.V
			u := s.pathNote(KindGlobalVar, "the value is read from the package variable "+g.Name, pos)
			u.Symbol = g.Name
			u.Candidates = s.eng.writersOf(g)
			u.global, u.globalPath = g, l.Path
			return u
		}
	}
	return nil
}

func (s *state) convert(in *sg.Instr) {
	dst, x := in.Dsts[0], in.Args[0]
	switch in.IR.Operator {
	case "slice_to_array_pointer":
		// aliases the slice
	case "change_interface":
		s.copyLocs(s.locs(x), []Loc{{own(dst), ""}}, "convert", in)
	case "make_interface":
		d := Loc{own(dst), ""}
		if pointerLike(x) {
			d.Path = "/*"
		}
		s.copyLocs(s.locs(x), []Loc{d}, "convert", in)
	default:
		switch {
		case pointerLike(dst) && pointerLike(x):
			// alias
		case pointerLike(dst):
			// e.g. string to []byte: the content becomes the elements
			s.put(Loc{own(dst), pathJoin("", "[]")}, s.flatten(s.locs(x)), s.whyFn(in, "convert"))
		case s.eng.opts.CharTaint && toOtherNumber(dst, x):
			// int(c): a number that is no character any more (-char-taint stops there)
			fs := s.flatten(s.locs(x))
			for i := range fs {
				fs[i].E = fs[i].E.WithChar(false)
			}
			s.put(Loc{own(dst), ""}, fs, s.whyFn(in, "convert"))
		default:
			s.copyLocs(s.locs(x), []Loc{{own(dst), ""}}, "convert", in)
		}
	}
}

// store writes val to the cells addr refers to (below the extra segment).
func (s *state) store(in *sg.Instr, addr []Loc, val *sg.Value, seg string, note *Uncertainty) {
	s.storeNoted(in, addr, val, seg, note, nil)
}

// storeAlias is store for a store whose address has locations that are there only because of ptrAlias.
func (s *state) storeAlias(in *sg.Instr, addr []Loc, val *sg.Value, alias map[Loc]bool) {
	s.storeNoted(in, addr, val, "", nil, alias)
}

// aliasSuffix is ptrAliasLocs(v) below a field.
func (s *state) aliasSuffix(v *sg.Value, field string) map[Loc]bool {
	al := s.ptrAliasLocs(v)
	if al == nil {
		return nil
	}
	out := make(map[Loc]bool, len(al))
	for l := range al {
		out[Loc{l.Root, pathJoin(l.Path, field)}] = true
	}
	return out
}

func (s *state) storeNoted(in *sg.Instr, addr []Loc, val *sg.Value, seg string, note *Uncertainty, alias map[Loc]bool) {
	var dst []Loc
	var dstAlias map[Loc]bool
	for _, l := range addr {
		p := l.Path
		if seg != "" {
			p = pathJoin(p, seg)
		}
		if pointerLike(val) {
			p = pathJoin(p, "*")
		}
		dst = append(dst, Loc{l.Root, p})
		if alias[l] {
			if dstAlias == nil {
				dstAlias = map[Loc]bool{}
			}
			dstAlias[Loc{l.Root, p}] = true
		}
	}
	s.copyLocsAlias(s.locs(val), dst, nil, dstAlias, "store", in, note)
}

// fieldSources applies field rules: reading a field that is declared a
// source introduces taint.
func (s *state) fieldSources(in *sg.Instr, base []Loc, field string) {
	for _, site := range s.eng.sites.ByIRInstr[sites.IRInstr{Func: s.fn.IR.ID, Block: in.IRBlock, Instr: in.IRIndex}] {
		if site.Role != sites.RoleSource {
			continue
		}
		pos := in.AddrPos
		if pos.IsZero() {
			pos = in.Pos
		}
		for _, l := range base {
			for _, label := range site.Labels {
				s.add(Loc{l.Root, pathJoin(l.Path, field)}, RealElem(label, site.ID),
					s.origin(pos, "source: field "+site.Symbol))
			}
		}
	}
}

// reservedShapeOK reports whether a call of one of the reserved symbols has the
// arguments and results that callInstr reads.
func reservedShapeOK(in *sg.Instr) bool {
	na, nd := len(in.Args), len(in.Dsts)
	switch in.Callee.Symbol {
	case symChanSend:
		return na >= 2
	case symChanRecv, symRangeStart:
		return na >= 1 && nd >= 1
	case symRangeNext, symRangeNextS:
		return na >= 1 && nd >= 3
	}
	return true
}

// toOtherNumber reports whether a conversion makes a number that is not a character of a number that may be one: a
// conversion between two number types where the result is not a byte, a rune or a char (int(c), float64(c)). byte(r)
// and rune(b) keep the character. By the name of the type, like the rest of isCharRead (to be replaced by an
// attribute of the extractor).
func toOtherNumber(dst, x *sg.Value) bool {
	if dst.Class != ir.ClassNumber || x.Class != ir.ClassNumber || dst.Type == x.Type {
		return false
	}
	switch string(dst.Type) {
	case "byte", "uint8", "rune", "int32", "char":
		return false
	}
	return true
}

// tableLookup is the part of an index_load that -char-taint adds: a value looked up by an index that is a character (or was
// made of one: a table of digits indexed by a byte, `digits[c>>4]`, an alphabet indexed by a value of six bits made of
// bytes) is made of that character: the table is how the text is rewritten (hex, base64). Where the index is not one, it
// is as it was (an index is not data).
func (s *state) tableLookup(in *sg.Instr) {
	if len(in.Args) < 2 || in.Args[1] == nil || pointerLike(in.Dsts[0]) {
		return
	}
	var fs []found
	for _, f := range s.flatten(s.locs(in.Args[1])) {
		if f.E.char {
			fs = append(fs, f)
		}
	}
	if len(fs) > 0 {
		base := s.whyFn(in, "table lookup by a character")
		// the step says so (char_transform), whatever the type of what was looked up (a byte, or a text from a map)
		s.put(Loc{own(in.Dsts[0]), ""}, fs, func(f found) *why { return s.withCharNote(base(f)) })
	}
}
