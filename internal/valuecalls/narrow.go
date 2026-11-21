package valuecalls

// The candidates of a call through Fn::call (Rust) are, as the extractor makes them, every closure whose
// arguments have the types the call passes (XTALK). Here they are narrowed to the closures that can be the
// receiver of the call: the functions that flow to it.
//
// The rule is "only when everything was followed" (docs/DESIGN-capture.md, the narrowing by the origin of the
// receiver). A narrowing can only lose a flow, never invent one, so the walk gives up, and the call keeps every
// candidate it had, at the first thing it cannot follow: an instruction it has no rule for, a field of a
// struct, a result of a call, a global, a trait object; a parameter whose callers are not all known; a
// limit (more than `most` functions, deeper than `maxDepth` calls). What is followed:
//
//   - the alloc of the environment of a closure (the function at the place its type names, ir.ClosureIndex) and
//     a make_closure: the function;
//   - a copy, a convert, a φ: what it is made of;
//   - a cell (an alloc that the function takes the address of: `&f` of a parameter `f`) that is only stored
//     to, loaded from, copied and given as the receiver of a call of a closure: the values that are stored;
//   - a field of the environment of a closure (named for the closure, so that the stores of it are the stores to
//     that name, in any function), when nothing takes its address to write through it;
//   - a parameter of a function whose callers are all known: it is not a closure (std calls those with
//     calls that are not in the code), not a method of a trait or called through one, not used as a value
//     (a pointer to it is a caller that is not a call); and the program does not start anywhere that the
//     callers are not all in what is read (Func.Private, or Func.Entry in the program and no
//     Func.ExternalEntry in it: the closed world).

import (
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irssa"
)

// whyNot is why a call kept its candidates.
const (
	notOpen    = "open_callers"
	notUnknown = "unknown_origin"
	notLimit   = "limit"
)

func isFnCall(c *ir.Callee) bool {
	if c == nil || c.Kind != ir.CalleeDynamic {
		return false
	}
	switch c.Symbol {
	case "std::ops::Fn::call", "std::ops::FnMut::call_mut", "std::ops::FnOnce::call_once":
		return true
	}
	return false
}

type narrowing struct {
	in    *ir.Instr
	keep  []ir.FuncID
	basis string
	from  string
	param map[ir.FuncID]int
	// same: every candidate flows to the receiver, so nothing is left out (the parameters are still told)
	same bool
}

type narrower struct {
	r      *resolver
	idx    *ir.ClosureIndex
	closed bool
	// valueRef: the functions that a make_closure names (used as a value: a call of them is not a caller we see).
	valueRef map[ir.FuncID]bool
	// inCands: the functions that a dynamic call that is not a call of a closure may reach.
	inCands map[ir.FuncID]bool
	fields  map[string]*fieldInfo
	indexed bool
	cells   map[cellKey]*cellInfo
	stats   ir.NarrowStats
}

type fieldInfo struct {
	stores  []stored
	mutated bool
}

type stored struct {
	fn   *ir.Func
	b, i int
}

type cellKey struct {
	fn  *ir.Func
	ref irssa.Ref
}

type cellInfo struct {
	values []irssa.Ref
	ok     bool
}

// narrowCalls narrows the candidates of the calls of closures of the program. The callers index of the resolver
// is the one of the candidates as the extractor made them: every decision is taken on it, and applied after.
func narrowCalls(r *resolver) {
	prog := r.prog
	any := false
	for _, f := range prog.Funcs {
		if ir.IsClosureID(string(f.ID)) {
			any = true
			break
		}
	}
	if !any {
		return
	}
	n := &narrower{r: r, idx: ir.NewClosureIndex(prog.Funcs), valueRef: map[ir.FuncID]bool{}, inCands: map[ir.FuncID]bool{},
		fields: map[string]*fieldInfo{}, cells: map[cellKey]*cellInfo{}}
	// the closed world: the program starts somewhere (main), nothing in it is called from outside by its symbol, and the
	// extractor says it marks what is (without that, "no mark" says nothing)
	entry, external := false, false
	for _, f := range prog.Funcs {
		external = external || f.ExternalEntry
		entry = entry || (f.Entry && !f.Init)
	}
	n.closed = entry && !external && prog.CallerMarks
	if r.callers == nil {
		r.indexCallers()
	}
	var calls []*callSite
	for _, f := range prog.Funcs {
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				switch {
				case in.Op == ir.OpMakeClosure:
					n.valueRef[in.Func] = true
				case in.Op == ir.OpCall && in.Callee != nil && in.Callee.Kind == ir.CalleeDynamic && !isFnCall(in.Callee):
					for _, c := range in.Callee.Candidates {
						n.inCands[c] = true
					}
				}
				if in.Op == ir.OpCall && isFnCall(in.Callee) && len(in.Args) > 0 && len(in.Callee.Candidates) > 1 {
					calls = append(calls, &callSite{f, bi, ii})
				}
			}
		}
	}
	var done []narrowing
	for _, c := range calls {
		in := &c.fn.Blocks[c.b].Instrs[c.i]
		if nr, why := n.decide(c, in); why != "" {
			switch why {
			case notOpen:
				n.stats.OpenCallers++
			case notLimit:
				n.stats.Limit++
			default:
				n.stats.UnknownOrigin++
			}
		} else if nr != nil {
			done = append(done, *nr)
		}
	}
	for _, d := range done {
		if d.same {
			d.in.Callee.CandParam = d.param
			continue
		}
		n.stats.Narrowed++
		n.stats.CandidatesOld += len(d.in.Callee.Candidates)
		n.stats.CandidatesNew += len(d.keep)
		d.in.Callee.NarrowedFrom = len(d.in.Callee.Candidates)
		d.in.Callee.NarrowBasis = d.basis
		d.in.Callee.NarrowFrom = d.from
		d.in.Callee.CandParam = d.param
		d.in.Callee.Candidates = d.keep
	}
	prog.Narrowing = &n.stats
}

type trace struct {
	n     *narrower
	found map[ir.FuncID]bool
	// home is the function that holds the call; cur is the parameter of it that the walk is going through
	// (-1 when it is not); via is, for each function found, that parameter if every way to it goes through
	// the same one, else -1: the callers that pass the parameter know which function they gave.
	home  *ir.Func
	cur   int
	via   map[ir.FuncID]int
	fail  string
	basis string
	named []string
	seen  map[seenKey]bool
}

// found records that the value may be the function id, through the parameter the walk is in.
func (t *trace) add(id ir.FuncID) {
	t.found[id] = true
	if p, ok := t.via[id]; !ok {
		t.via[id] = t.cur
	} else if p != t.cur {
		t.via[id] = -1
	}
}

func (t *trace) stop(why string) {
	if t.fail == "" {
		t.fail = why
	}
}

func (n *narrower) decide(c *callSite, in *ir.Instr) (*narrowing, string) {
	form := n.r.form(c.fn)
	if form == nil {
		return nil, notUnknown
	}
	t := &trace{n: n, found: map[ir.FuncID]bool{}, seen: map[seenKey]bool{}, home: c.fn, cur: -1, via: map[ir.FuncID]int{}}
	t.origins(c.fn, form.Operand(c.b, c.i, 0), 0)
	if t.fail != "" {
		return nil, t.fail
	}
	if len(t.found) > most {
		return nil, notLimit
	}
	var keep []ir.FuncID
	for _, id := range in.Callee.Candidates {
		if t.found[id] {
			keep = append(keep, id)
		}
	}
	if len(keep) == 0 {
		// what flows here is not among the candidates the extractor found (a type it could not tell): not a reason to
		// call nothing
		return nil, notUnknown
	}
	sort.Slice(keep, func(i, j int) bool { return keep[i] < keep[j] })
	from := "the closure is made in the function"
	if len(t.named) > 0 {
		sort.Strings(t.named)
		from = "the function given as " + strings.Join(dedup(t.named), ", ") + ", from the callers that pass it"
	}
	var param map[ir.FuncID]int
	for _, id := range keep {
		if p, ok := t.via[id]; ok && p >= 0 {
			if param == nil {
				param = map[ir.FuncID]int{}
			}
			param[id] = p
		}
	}
	return &narrowing{in: in, keep: keep, basis: t.basis, from: from, param: param, same: len(keep) == len(in.Callee.Candidates)}, ""
}

func dedup(s []string) []string {
	var out []string
	for i, x := range s {
		if i == 0 || x != s[i-1] {
			out = append(out, x)
		}
	}
	return out
}

// origins collects the functions that the value at ref may be.
func (t *trace) origins(f *ir.Func, ref irssa.Ref, depth int) {
	if t.fail != "" {
		return
	}
	if depth > maxDepth {
		t.stop(notLimit)
		return
	}
	key := seenKey{f, ref}
	if t.seen[key] {
		return
	}
	t.seen[key] = true
	form := t.n.r.form(f)
	if form == nil {
		t.stop(notUnknown)
		return
	}
	switch ref.Kind {
	case irssa.RefPhi:
		found := false
		for _, ph := range form.Phis[ref.Block] {
			if ph.Var == ref.Var {
				found = true
				for _, a := range ph.Args {
					t.origins(f, a, depth)
				}
			}
		}
		if !found {
			t.stop(notUnknown)
		}
		return
	case irssa.RefInstr:
	default:
		t.stop(notUnknown)
		return
	}
	in := &f.Blocks[ref.Block].Instrs[ref.Index]
	switch in.Op {
	case ir.OpMakeClosure:
		t.add(in.Func)
	case ir.OpAlloc:
		if ir.IsEnvType(string(in.Type)) {
			cl := t.n.idx.Lookup(string(in.Type))
			if cl == nil {
				t.stop(notUnknown)
				return
			}
			t.add(cl.ID)
			return
		}
		t.cell(f, form, ref, depth)
	case ir.OpParam:
		if in.Index < declared(f) {
			t.param(f, in.Index, depth)
		} else {
			t.stop(notUnknown)
		}
	case ir.OpCopy:
		if in.View || len(in.Args) != 1 {
			t.stop(notUnknown)
			return
		}
		t.origins(f, form.Operand(ref.Block, ref.Index, 0), depth)
	case ir.OpConvert:
		if len(in.Args) != 1 {
			t.stop(notUnknown)
			return
		}
		t.origins(f, form.Operand(ref.Block, ref.Index, 0), depth)
	case ir.OpLoad:
		// the content of what it points to: the pointer and the closure are not told apart (a pointer to a closure
		// has the closure's functions)
		t.origins(f, form.Operand(ref.Block, ref.Index, 0), depth)
	case ir.OpFieldLoad:
		t.field(in.Field, depth)
	case ir.OpAddrOf:
		// the address of a field of the environment of a closure (a call through `&self` of a closure kept in one)
		if len(in.Path) == 1 && in.Path[0].Kind == ir.StepField && in.Global == "" && len(in.Args) == 1 {
			t.field(in.Path[0].Field, depth)
			return
		}
		t.stop(notUnknown)
	case ir.OpCall:
		// the copy of a closure is the closure
		if in.Callee != nil && in.Callee.Kind == ir.CalleeStatic && strings.HasPrefix(string(in.Callee.Func), "<{closure@") &&
			strings.HasSuffix(string(in.Callee.Func), " as std::clone::Clone>::clone") && len(in.Args) == 1 {
			t.origins(f, form.Operand(ref.Block, ref.Index, 0), depth)
			return
		}
		// the copy of a function that a generic helper takes (`f.clone()` with `F: Fn + Clone`): the extractor cannot name
		// the impl of a type parameter, so it is a dynamic call whose candidates say nothing. What a `Clone` of a callable
		// gives is what it was given (a closure, a function, a pointer to one)
		if isDynamicClone(in) {
			t.origins(f, form.Operand(ref.Block, ref.Index, 0), depth)
			return
		}
		t.stop(notUnknown)
	default:
		t.stop(notUnknown)
	}
}

func isDynamicClone(in *ir.Instr) bool {
	return in.Op == ir.OpCall && in.Callee != nil && in.Callee.Kind == ir.CalleeDynamic && in.Callee.Symbol == "std::clone::Clone::clone" && len(in.Args) == 1
}

// isCloneOfCallable: a clone that the walk follows (origins): it reads what its argument points to, and gives it back.
func isCloneOfCallable(in *ir.Instr) bool {
	if isDynamicClone(in) {
		return true
	}
	return in.Op == ir.OpCall && in.Callee != nil && in.Callee.Kind == ir.CalleeStatic && strings.HasPrefix(string(in.Callee.Func), "<{closure@") &&
		strings.HasSuffix(string(in.Callee.Func), " as std::clone::Clone>::clone") && len(in.Args) == 1
}

func isClosureField(f string) bool { return strings.Contains(f, "{closure#") }

// field follows a field of the environment of a closure: what is stored to the field of that name.
func (t *trace) field(name string, depth int) {
	if !isClosureField(name) {
		t.stop(notUnknown)
		return
	}
	fi := t.n.fieldInfo(name)
	if fi.mutated || len(fi.stores) == 0 {
		t.stop(notUnknown)
		return
	}
	for _, s := range fi.stores {
		form := t.n.r.form(s.fn)
		if form == nil {
			t.stop(notUnknown)
			return
		}
		t.origins(s.fn, form.Operand(s.b, s.i, 1), depth) // a field of an environment is not a call up: the depth counts the callers that are followed
		if t.fail != "" {
			return
		}
	}
}

// fieldInfo lists, for a field of the environment of a closure, the stores to it, and whether an address of it is
// used to write (the field is then not only what the stores say).
func (n *narrower) fieldInfo(name string) *fieldInfo {
	n.indexFields()
	if fi := n.fields[name]; fi != nil {
		return fi
	}
	return &fieldInfo{}
}

func (n *narrower) indexFields() {
	if n.indexed {
		return
	}
	n.indexed = true
	for _, f := range n.r.prog.Funcs {
		var form *irssa.Form
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				switch {
				case in.Op == ir.OpFieldStore && isClosureField(in.Field):
					fi := n.fields[in.Field]
					if fi == nil {
						fi = &fieldInfo{}
						n.fields[in.Field] = fi
					}
					fi.stores = append(fi.stores, stored{f, bi, ii})
				case in.Op == ir.OpAddrOf && len(in.Path) >= 1 && in.Path[len(in.Path)-1].Kind == ir.StepField && isClosureField(in.Path[len(in.Path)-1].Field):
					name := in.Path[len(in.Path)-1].Field
					fi := n.fields[name]
					if fi == nil {
						fi = &fieldInfo{}
						n.fields[name] = fi
					}
					if len(in.Path) != 1 {
						fi.mutated = true
						continue
					}
					if form == nil {
						form = n.r.form(f)
					}
					if form == nil {
						fi.mutated = true
						continue
					}
					ref := irssa.Ref{Kind: irssa.RefInstr, Block: bi, Index: ii, Dst: 0}
					if _, _, ok := n.aliasUses(f, form, ref, false); !ok && !sharedPointer(f, in) {
						fi.mutated = true
					}
				}
			}
		}
	}
}

// sharedLocal: the local is typed as a shared reference (`&T`, not `&mut T`).
func sharedLocal(f *ir.Func, id ir.LocalID) bool {
	if int(id) < 0 || int(id) >= len(f.Locals) {
		return false
	}
	t := string(f.Locals[id].Type)
	return strings.HasPrefix(t, "&") && !strings.HasPrefix(t, "&mut ")
}

// sharedPointer: the address is a shared reference (`&F`, a copy of the address is typed so): nothing can be written
// through it (a function value has no interior that makes it another function), so where the address goes (stored in
// the environment of a closure, given to a call) is not a way to change the field.
func sharedPointer(f *ir.Func, addr *ir.Instr) bool {
	if len(addr.Dsts) != 1 {
		return false
	}
	cur := addr.Dsts[0]
	if sharedLocal(f, cur) {
		return true
	}
	// the copy of the address (the address is made into a local, and copied to the one that has the type)
	found := false
	for bi := range f.Blocks {
		for ii := range f.Blocks[bi].Instrs {
			in := &f.Blocks[bi].Instrs[ii]
			if in.Op == ir.OpCopy && len(in.Args) == 1 && len(in.Dsts) == 1 && in.Args[0] == cur {
				if in.View || !sharedLocal(f, in.Dsts[0]) {
					return false // a copy that is not a shared reference (a `&mut`, a raw pointer, untyped): it may write
				}
				found = true
			}
		}
	}
	return found
}

// aliasUses looks at what a function does with a pointer: it is copied (the copy is the same pointer), loaded
// from, converted, or the receiver of a call of a closure; it is stored to (the values are returned, when
// stores are allowed); anything else (given to another function, stored as a value, returned, a field of it
// taken, written through) lets it escape, and ok is false.
func (n *narrower) aliasUses(f *ir.Func, form *irssa.Form, start irssa.Ref, allowStore bool) (values []irssa.Ref, aliases map[irssa.Ref]bool, ok bool) {
	aliases = map[irssa.Ref]bool{start: true}
	for changed := true; changed; {
		changed = false
		for bi := range f.Blocks {
			b := &f.Blocks[bi]
			for ii := range b.Instrs {
				in := &b.Instrs[ii]
				if form.IsPromotedAccess(*in) {
					continue
				}
				for k := range in.Args {
					if !aliases[form.Operand(bi, ii, k)] {
						continue
					}
					switch in.Op {
					case ir.OpLoad:
						// reads it
					case ir.OpStore:
						if k != 0 || !allowStore {
							return nil, nil, false
						}
					case ir.OpCopy, ir.OpConvert:
						if (in.Op == ir.OpCopy && in.View) || len(in.Args) != 1 {
							return nil, nil, false
						}
						d := irssa.Ref{Kind: irssa.RefInstr, Block: bi, Index: ii, Dst: 0}
						if !aliases[d] {
							aliases[d] = true
							changed = true
						}
					case ir.OpCall:
						if !(k == 0 && (isFnCall(in.Callee) || isCloneOfCallable(in))) {
							return nil, nil, false
						}
					default:
						return nil, nil, false
					}
				}
			}
			for k := range b.Term.Values {
				if aliases[form.Operand(bi, len(b.Instrs), k)] {
					return nil, nil, false
				}
			}
		}
	}
	if allowStore {
		for bi := range f.Blocks {
			b := &f.Blocks[bi]
			for ii := range b.Instrs {
				in := &b.Instrs[ii]
				if in.Op == ir.OpStore && !form.IsPromotedAccess(*in) && aliases[form.Operand(bi, ii, 0)] {
					values = append(values, form.Operand(bi, ii, 1))
				}
			}
		}
	}
	return values, aliases, true
}

// cell follows a pointer to a variable that the function takes the address of: what is stored to it.
func (t *trace) cell(f *ir.Func, form *irssa.Form, ref irssa.Ref, depth int) {
	key := cellKey{f, ref}
	ci := t.n.cells[key]
	if ci == nil {
		ci = &cellInfo{}
		vals, _, ok := t.n.aliasUses(f, form, ref, true)
		ci.values, ci.ok = vals, ok && len(vals) > 0
		t.n.cells[key] = ci
	}
	if !ci.ok {
		t.stop(notUnknown)
		return
	}
	for _, v := range ci.values {
		t.origins(f, v, depth)
		if t.fail != "" {
			return
		}
	}
}

// param follows a parameter to the arguments of its callers, when the callers are all known.
func (t *trace) param(f *ir.Func, idx, depth int) {
	n := t.n
	switch {
	case ir.IsClosureID(string(f.ID)), strings.HasPrefix(string(f.ID), "<"), n.inCands[f.ID], n.valueRef[f.ID], f.ExternalEntry:
		t.stop(notOpen)
		return
	case f.Private:
	case n.closed:
		t.basis = ir.BasisClosedWorldEntry
	default:
		t.stop(notOpen)
		return
	}
	cs := n.r.callers[f.ID]
	if len(cs) == 0 {
		t.stop(notUnknown)
		return
	}
	pname := ""
	if idx < len(f.Params) {
		pname = f.Params[idx].Name
	}
	if pname == "" {
		pname = "an argument"
	}
	t.named = append(t.named, pname)
	if depth == 0 && f == t.home {
		old := t.cur
		t.cur = idx
		defer func() { t.cur = old }()
	}
	for _, c := range cs {
		cin := &c.fn.Blocks[c.b].Instrs[c.i]
		off := 0
		if cin.Callee.Kind == ir.CalleeValue {
			off = 1
		}
		k := idx + off
		if k >= len(cin.Args) {
			t.stop(notUnknown)
			return
		}
		cf := n.r.form(c.fn)
		if cf == nil {
			t.stop(notUnknown)
			return
		}
		t.origins(c.fn, cf.Operand(c.b, c.i, k), depth+1)
		if t.fail != "" {
			return
		}
	}
}
