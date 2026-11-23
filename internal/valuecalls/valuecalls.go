// Package valuecalls finds the functions a call of a function value may
// reach, on the IR, so that everything that reads the program sees the same
// ones: the enumeration of the sites (a rule can name the function that a
// variable holds: `rm := os.Remove; rm(p)`) and the graph the analysis runs on.
//
// A call of a function value has candidates in two ways. The extractor of Go
// gives the ones its call graph (VTA) found. For the others, the functions that
// flow to the value are found here: through copies, φ functions and the
// parameters of the function (the arguments its callers pass). It follows no
// memory: a function kept in a field or a list is not found, and the call stays
// unresolved.
//
// The function that holds the call may reach the value only through one of its
// parameters. Which candidate is called then depends on the caller (the call
// graph does not tell the callers apart), so the candidate says which parameter
// it came through (ValueCand.Param): the analysis keeps that with the flow, and
// the caller, which knows what it passed, can leave out the flows that go
// through another function (ADR-045).
package valuecalls

import (
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irssa"
)

// most is the number of candidates beyond which a call is left unresolved: a
// value that many functions flow to says little.
const most = ir.ValueCandidateMost

// maxDepth is how many calls up the arguments are followed.
const maxDepth = 3

// Resolve sets ValueCands of the calls of function values. It does it once.
func Resolve(src ir.Source) {
	if src == nil {
		return
	}
	prog := src.Header()
	if prog == nil || prog.ValuesResolved {
		return
	}
	prog.ValuesResolved = true
	r := &resolver{src: src, prog: prog, forms: map[*ir.Func]*irssa.Form{}}
	ir.Walk(src, func(_ ir.Ref, f *ir.Func) {
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				if in.Op != ir.OpCall || in.Callee == nil || in.Callee.Kind != ir.CalleeValue || len(in.Args) == 0 {
					continue
				}
				r.resolve(f, bi, ii, in)
			}
		}
	})
	narrowCalls(r)
	r.forms = nil
	r.callers = nil
	r.makers = nil
}

type callSite struct {
	fn   *ir.Func
	b, i int
}

type resolver struct {
	src     ir.Source
	prog    *ir.Program // the header of src: what is not a function
	forms   map[*ir.Func]*irssa.Form
	callers map[ir.FuncID][]callSite
	// makers lists where each closure is made (the make_closure instructions that name it): what a captured variable holds is
	// what the instruction was given
	makers map[ir.FuncID][]callSite
	// cut is set when a walk up the callers stopped at maxDepth
	cut bool
	// partial holds the reasons why a walk could not follow every caller's value (the functions it
	// found are then not known to be all of them)
	partial map[string]bool
}

func (r *resolver) notePartial(why string) {
	if r.partial == nil {
		r.partial = map[string]bool{}
	}
	r.partial[why] = true
}

func (r *resolver) form(f *ir.Func) *irssa.Form {
	if fm, ok := r.forms[f]; ok {
		return fm
	}
	fm, err := irssa.Build(f)
	if err != nil {
		fm = nil
	}
	r.forms[f] = fm
	return fm
}

func (r *resolver) candOf(id ir.FuncID, via int) ir.ValueCand {
	c := ir.ValueCand{ID: id, Symbol: string(id), Param: via}
	if h, ok := ir.HeadOf(r.src, id); ok {
		c.Symbol, c.Recv = h.Symbol, h.Recv
	} else {
		c.Recv = strings.HasPrefix(string(id), "(")
	}
	return c
}

func (r *resolver) resolve(fn *ir.Func, bi, ii int, in *ir.Instr) {
	form := r.form(fn)
	if form == nil {
		return
	}
	root := form.Operand(bi, ii, 0)
	if id := r.directFunc(fn, bi, ii); id != "" {
		// The value is a function (or a closure made right there): the graph
		// calls it, whatever the call graph of the extractor says.
		in.Callee.ValueCands = []ir.ValueCand{r.candOf(id, -1)}
		return
	}
	if cs := in.Callee.Candidates; len(cs) > 0 {
		// The extractor's call graph says. Which parameter they came through is
		// worth knowing only when there are several.
		via := -1
		if len(cs) > 1 {
			via = r.onlyParam(fn, form, root)
		}
		for _, id := range cs {
			in.Callee.ValueCands = append(in.Callee.ValueCands, r.candOf(id, via))
		}
		return
	}
	if r.callers == nil {
		r.indexCallers()
	}
	found := map[ir.FuncID]*found{}
	r.cut = false
	r.partial = nil
	r.funcsOf(fn, root, fn, found, map[seenKey]bool{}, 0, -1)
	if len(found) > most {
		// the functions are known, rivus leaves them out: not "the callee is not known"
		in.Callee.ValueLimit, in.Callee.ValueFound = ir.ValueCandidateLimit, len(found)
		if p := r.onlyParam(fn, form, root); p >= 0 {
			// the value came through the parameter p: a caller that passes a function there is told (taint, LimitedParams)
			in.Callee.ValueLimitParams = []int{p}
		}
		return
	}
	if len(found) == 0 {
		if r.cut {
			in.Callee.ValueLimit = ir.ValueCallerDepthLimit
		} else if p := r.paramThroughFields(fn, form, root); p >= 0 {
			// no function is known, and the value is what a parameter holds: a caller that passes something there is told
			// (taint, UnresolvedParams)
			in.Callee.ValueUnresolvedParams = []int{p}
		}
		return
	}
	if r.cut {
		r.notePartial(ir.ValueWhyDepth)
	}
	if len(r.partial) > 0 {
		// The functions that were found stay the candidates (nothing that is found now is lost), but
		// they are not said to be all of them: the call has an uncertainty, and nothing narrows the
		// candidates because it looks as if they were every one.
		in.Callee.ValueLimit, in.Callee.ValueFound = ir.ValuePartial, len(found)
		for why := range r.partial {
			in.Callee.ValueWhy = append(in.Callee.ValueWhy, why)
		}
		sort.Strings(in.Callee.ValueWhy)
	}
	ids := make([]string, 0, len(found))
	for id := range found {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		f := found[ir.FuncID(id)]
		c := r.candOf(ir.FuncID(id), f.via)
		c.Bind = f.bind
		in.Callee.ValueCands = append(in.Callee.ValueCands, c)
	}
}

// indexCallers lists the calls that reach each function: the static ones and
// the candidates of the others.
func (r *resolver) indexCallers() {
	r.callers = map[ir.FuncID][]callSite{}
	ir.Walk(r.src, func(_ ir.Ref, f *ir.Func) {
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				if in.Op != ir.OpCall || in.Callee == nil {
					continue
				}
				if in.Callee.Func != "" {
					r.callers[in.Callee.Func] = append(r.callers[in.Callee.Func], callSite{f, bi, ii})
				}
				if in.Callee.Kind == ir.CalleeValue && len(in.Callee.Candidates) == 0 && len(in.Args) > 0 {
					// a function held in a variable and called: it is the callee
					if id := r.directFunc(f, bi, ii); id != "" {
						r.callers[id] = append(r.callers[id], callSite{f, bi, ii})
					}
				}
				for _, c := range in.Callee.Candidates {
					r.callers[c] = append(r.callers[c], callSite{f, bi, ii})
				}
			}
		}
	})
}

// indexMakers lists the make_closure instructions of each closure.
func (r *resolver) indexMakers() {
	r.makers = map[ir.FuncID][]callSite{}
	ir.Walk(r.src, func(_ ir.Ref, f *ir.Func) {
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				if in.Op == ir.OpMakeClosure {
					r.makers[in.Func] = append(r.makers[in.Func], callSite{f, bi, ii})
				}
			}
		}
	})
}

// directFunc is the function that a call of a function value calls, when the
// value is a function (or a closure made right there), perhaps copied.
func (r *resolver) directFunc(f *ir.Func, bi, ii int) ir.FuncID {
	form := r.form(f)
	if form == nil {
		return ""
	}
	ref := form.Operand(bi, ii, 0)
	for i := 0; i < 8; i++ {
		k, in, from := r.classify(f, form, ref)
		switch {
		case k == refClosure:
			return in.Func
		case k == refFrom && ref.Kind == irssa.RefInstr && len(from) == 1:
			ref = from[0]
		default:
			return ""
		}
	}
	return ""
}

type found struct {
	via  int
	bind *ir.Binding
}

type seenKey struct {
	fn  *ir.Func
	ref irssa.Ref
}

// declared is the number of parameters that are declared (the others are
// captured variables).
func declared(f *ir.Func) int {
	n := 0
	for _, p := range f.Params {
		if !p.Captured {
			n++
		}
	}
	return n
}

// step looks at what a reference is: a function (a closure, with the
// instruction that made it), a parameter, or something it comes from (a copy,
// a φ). Anything else is nothing the analysis can follow.
type refKind int

const (
	refOther refKind = iota
	refClosure
	refParam
	refFrom
	refCaptured
)

func (r *resolver) classify(f *ir.Func, form *irssa.Form, ref irssa.Ref) (refKind, *ir.Instr, []irssa.Ref) {
	switch ref.Kind {
	case irssa.RefPhi:
		for _, ph := range form.Phis[ref.Block] {
			if ph.Var == ref.Var {
				return refFrom, nil, ph.Args
			}
		}
	case irssa.RefInstr:
		in := &f.Blocks[ref.Block].Instrs[ref.Index]
		switch in.Op {
		case ir.OpMakeClosure:
			return refClosure, in, nil
		case ir.OpParam:
			if in.Index < declared(f) {
				return refParam, in, nil
			}
			return refCaptured, in, nil
		case ir.OpCopy:
			if !in.View && len(in.Args) == 1 {
				return refFrom, nil, []irssa.Ref{form.Operand(ref.Block, ref.Index, 0)}
			}
		case ir.OpConvert:
			// a function made a value of another function type (a function item
			// that becomes a function pointer): the same function
			if len(in.Args) == 1 && len(in.Dsts) == 1 && f.Locals[in.Dsts[0]].Class == ir.ClassFunc && f.Locals[in.Args[0]].Class == ir.ClassFunc {
				return refFrom, nil, []irssa.Ref{form.Operand(ref.Block, ref.Index, 0)}
			}
		}
	}
	return refOther, nil, nil
}

// onlyParam is the parameter of the function that the value is, through copies
// and φ functions, and nothing else; -1 if it is anything else too.
func (r *resolver) onlyParam(fn *ir.Func, form *irssa.Form, root irssa.Ref) int {
	param := -1
	ok := true
	seen := map[irssa.Ref]bool{}
	var walk func(ref irssa.Ref)
	walk = func(ref irssa.Ref) {
		if seen[ref] || !ok {
			return
		}
		seen[ref] = true
		k, in, from := r.classify(fn, form, ref)
		switch k {
		case refParam:
			if param >= 0 && param != in.Index {
				ok = false
			}
			param = in.Index
		case refFrom:
			for _, a := range from {
				walk(a)
			}
		default:
			ok = false
		}
	}
	walk(root)
	if !ok {
		return -1
	}
	return param
}

// funcsOf collects the functions that reach a value. via is the parameter of
// home that the walk has gone through (-1 before it has).
func (r *resolver) funcsOf(f *ir.Func, ref irssa.Ref, home *ir.Func, out map[ir.FuncID]*found, seen map[seenKey]bool, depth, via int) {
	if depth > maxDepth {
		r.cut = true
		return
	}
	key := seenKey{f, ref}
	if seen[key] {
		return
	}
	seen[key] = true
	form := r.form(f)
	if form == nil {
		r.notePartial(ir.ValueWhyNoForm)
		return
	}
	k, in, from := r.classify(f, form, ref)
	switch k {
	case refOther:
		r.notePartial(ir.ValueWhyUntraced)
	case refClosure:
		id := in.Func
		cur, ok := out[id]
		if !ok {
			cur = &found{via: via}
			out[id] = cur
		} else if cur.via != via {
			cur.via = -1
		}
		// The values it captured go with it when the closure is made in the
		// function that calls it (they are values of that function); from
		// another function they are not at hand.
		if f == home && len(in.Args) > 0 && cur.bind == nil {
			cur.bind = &ir.Binding{Block: ref.Block, Instr: ref.Index}
		}
	case refFrom:
		for _, a := range from {
			r.funcsOf(f, a, home, out, seen, depth, via)
		}
	case refCaptured:
		// a variable that the closure took from the function that made it: the value that the make_closure was given
		if r.makers == nil {
			r.indexMakers()
		}
		k := in.Index - declared(f)
		for _, m := range r.makers[f.ID] {
			min := &m.fn.Blocks[m.b].Instrs[m.i]
			if len(min.Args) != len(f.Params)-declared(f) {
				// the extractor left out a variable it did not find: the places do not tell which is which
				r.notePartial(ir.ValueWhyArgs)
				continue
			}
			mf := r.form(m.fn)
			if mf == nil {
				r.notePartial(ir.ValueWhyNoForm)
				continue
			}
			r.funcsOf(m.fn, mf.Operand(m.b, m.i, k), home, out, seen, depth+1, via)
		}
	case refParam:
		if depth == 0 && f == home {
			via = in.Index
		}
		for _, c := range r.callers[f.ID] {
			cin := &c.fn.Blocks[c.b].Instrs[c.i]
			off := 0
			if cin.Callee.Kind == ir.CalleeValue {
				off = 1
			}
			k := in.Index + off
			if k >= len(cin.Args) {
				r.notePartial(ir.ValueWhyArgs)
				continue
			}
			cf := r.form(c.fn)
			if cf == nil {
				r.notePartial(ir.ValueWhyNoForm)
				continue
			}
			r.funcsOf(c.fn, cf.Operand(c.b, c.i, k), home, out, seen, depth+1, via)
		}
	}
}

// paramThroughFields is onlyParam for a value that may also be read from a field, an element or a cell of what a
// parameter holds (a function kept in an object, a list or a map that was passed): the parameter is the one that holds
// it. -1 when there is not one such parameter.
func (r *resolver) paramThroughFields(fn *ir.Func, form *irssa.Form, root irssa.Ref) int {
	w := &fieldWalk{r: r, fn: fn, form: form, param: -1, ok: true, seen: map[irssa.Ref]bool{}}
	w.walk(root)
	if !w.ok {
		return -1
	}
	return w.param
}

// fieldWalk is the walk of paramThroughFields. (A method, not a closure that calls itself: the analysis of rivus by
// itself counts a closure that is called through a variable it captured as a capture it did not follow, TestDogfood.)
type fieldWalk struct {
	r     *resolver
	fn    *ir.Func
	form  *irssa.Form
	param int
	ok    bool
	seen  map[irssa.Ref]bool
}

func (w *fieldWalk) walk(ref irssa.Ref) {
	if w.seen[ref] || !w.ok {
		return
	}
	w.seen[ref] = true
	if ref.Kind == irssa.RefInstr {
		if in := &w.fn.Blocks[ref.Block].Instrs[ref.Index]; (in.Op == ir.OpFieldLoad || in.Op == ir.OpIndexLoad || in.Op == ir.OpLoad) && len(in.Args) > 0 {
			w.walk(w.form.Operand(ref.Block, ref.Index, 0))
			return
		}
	}
	k, in, from := w.r.classify(w.fn, w.form, ref)
	switch k {
	case refParam:
		if w.param >= 0 && w.param != in.Index {
			w.ok = false
		}
		w.param = in.Index
	case refFrom:
		for _, a := range from {
			w.walk(a)
		}
	default:
		w.ok = false
	}
}
