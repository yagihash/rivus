package sites

import (
	"fmt"
	"reflect"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irssa"
	"github.com/yagihash/rivus/internal/model"
)

// CallbackUnresolvedPrefix starts the detail of the unresolved_callee note of a call that a callback_param source names and whose function was
// not found: the summary counts the notes by it (summary.callback_param_unresolved).
const CallbackUnresolvedPrefix = "callback_param_unresolved:"

// callbackTarget is a function that a call of the symbol of a callback_param source is given, and the source.
type callbackTarget struct {
	src    model.CallbackParam
	symbol string
}

// callbackTargets finds, for every call of a symbol that a callback_param source names, the function that is the
// operand of the source: the parameters of that function are the source (docs/DESIGN-callback-sources.md). The function
// is what the operand is made of where the call is made: a function literal, a function that is named, a local that
// holds one of them, possibly through copies and conversions. What is not found (a φ of two functions, a field, a
// parameter of the caller, a result) is not followed, and the call is recorded, so that the analysis says that the
// parameters of the function it was given are not marked (unresolved_callee).
func callbackTargets(irsrc ir.Source, rules *model.Rules, res *Result) map[ir.FuncID][]callbackTarget {
	if len(rules.CallbackParams) == 0 {
		return nil
	}
	out := map[ir.FuncID][]callbackTarget{}
	w := &callerWalk{src: irsrc, res: res, forms: map[ir.FuncID]*readFunc{}}
	// only the functions of the project that call such a symbol are read (the Source knows the symbols of the calls)
	callsOne := func(seq int) bool {
		for _, sym := range ir.CallSymbols(irsrc, seq) {
			if len(rules.CallbackParams[sym]) > 0 {
				return true
			}
		}
		return false
	}
	ir.WalkProjectIf(irsrc, callsOne, func(_ ir.Ref, fn *ir.Func) bool {
		var form *irssa.Form
		for bi, b := range fn.Blocks {
			for ii, in := range b.Instrs {
				if in.Op != ir.OpCall || in.Callee.Kind == ir.CalleeValue {
					continue
				}
				srcs := rules.CallbackParams[in.Callee.Symbol]
				if in.Callee.Symbol == "" || len(srcs) == 0 {
					continue
				}
				hasRecv := in.Callee.Kind == ir.CalleeDynamic || (in.Callee.Kind == ir.CalleeStatic && in.Callee.Recv)
				for _, src := range srcs {
					at := *src.Operand
					if hasRecv && !src.SelfFirst {
						at++
					}
					if at >= len(in.Args) {
						res.noteCallback(IRInstr{fn.ID, bi, ii}, in.Callee.Symbol, *src.Operand, "not an operand of the call")
						continue
					}
					if form == nil {
						form, _ = irssa.Build(fn)
					}
					if form == nil {
						res.noteCallback(IRInstr{fn.ID, bi, ii}, in.Callee.Symbol, *src.Operand, "not known")
						continue
					}
					ids, why := w.functionsOf(fn, form, form.Operand(bi, ii, at), 0, map[paramKey]bool{})
					if why != "" {
						// some function the call may be given was not found: the call is said, and the ones that were found are marked
						res.noteCallback(IRInstr{fn.ID, bi, ii}, in.Callee.Symbol, *src.Operand, why)
					}
					for _, id := range ids {
						if _, ok := irsrc.Lookup(id); !ok {
							res.noteCallback(IRInstr{fn.ID, bi, ii}, in.Callee.Symbol, *src.Operand, "a function that is not in the program")
							continue
						}
						dup := false
						for _, t := range out[id] {
							if t.symbol == in.Callee.Symbol && t.src.Source.Origin == src.Source.Origin && reflect.DeepEqual(t.src.Source, src.Source) {
								dup = true
							}
						}
						if !dup {
							out[id] = append(out[id], callbackTarget{src: src, symbol: in.Callee.Symbol})
						}
					}
				}
			}
		}
		return true
	})
	return out
}

// functionOf follows a value to the function that makes it, through copies and conversions that keep it whole.
func functionOf(form *irssa.Form, r irssa.Ref, depth int, closures func() *ir.ClosureIndex) (ir.FuncID, string) {
	if depth > 8 {
		return "", "not known (a long chain of copies)"
	}
	switch r.Kind {
	case irssa.RefInstr:
	case irssa.RefPhi:
		return "", "one of several functions (a variable that more than one value is stored in)"
	default:
		return "", "not known"
	}
	in := form.Func.Blocks[r.Block].Instrs[r.Index]
	switch in.Op {
	case ir.OpMakeClosure:
		if in.Func == "" {
			return "", "not known"
		}
		return in.Func, ""
	case ir.OpCopy, ir.OpConvert:
		if in.View || len(in.Args) != 1 {
			return "", "not known"
		}
		return functionOf(form, form.Operand(r.Block, r.Index, 0), depth+1, closures)
	case ir.OpAlloc:
		// Rust: a closure is not a make_closure but an alloc of the type of its environment; the function is the one that the alloc
		// says, else the one at the place that the type names (ir.ClosureIndex.FuncOf). A caller that has no index (closures is nil)
		// takes the one that the alloc says.
		if ir.IsEnvType(string(in.Type)) {
			var ix *ir.ClosureIndex
			if closures != nil {
				ix = closures()
			}
			if id := ix.FuncOf(string(in.Type), in.Func); id != "" {
				return id, ""
			}
			if ix != nil {
				if _, n := ix.Lookup(string(in.Type)); n >= 2 {
					return "", fmt.Sprintf("one of %d closures written at the same place (a macro)", n)
				}
			}
		}
		return "", "not known"
	case ir.OpParam:
		// followed to what the callers pass (callerWalk.functionsOf); here only when that is not asked
		return "", "a parameter of the function that makes the call (a helper that registers what it is given)"
	case ir.OpCall:
		return "", "the result of a call"
	case ir.OpFieldLoad, ir.OpLoad, ir.OpIndexLoad:
		return "", "read from a field, a variable of another function or an element"
	}
	return "", "not known"
}

// callbackParamIndexes are the indexes in fn.Params of the parameters that the function declares, in order (a captured value is
// not one, nor the receiver), and the span of a target in them.
func callbackParamIndexes(fn *ir.Func) []int {
	var idx []int
	for i, p := range fn.Params {
		if !p.Captured {
			idx = append(idx, i)
		}
	}
	if fn.Recv && len(idx) > 0 {
		idx = idx[1:]
	}
	return idx
}

// span is the positions from..to (included) of a target among n declared parameters.
func (t callbackTarget) span(n int) (int, int) {
	from, to := 0, n-1
	if t.src.From != nil {
		from = *t.src.From
	}
	if t.src.To != nil && *t.src.To < to {
		to = *t.src.To
	}
	return from, to
}

// noteCallback records a call whose function operand was not found (the first reason stays).
func (res *Result) noteCallback(k IRInstr, symbol string, operand int, why string) {
	if res.UnresolvedCallbacks == nil {
		res.UnresolvedCallbacks = map[IRInstr]string{}
	}
	if _, ok := res.UnresolvedCallbacks[k]; !ok {
		res.UnresolvedCallbacks[k] = fmt.Sprintf(CallbackUnresolvedPrefix+" the function given to %s as operand %d is %s: its parameters that callback_param marks as sources are not marked here (a function written at the call is followed)", symbol, operand, why)
	}
}

// maxCallerDepth is how many calls up the arguments are followed (the same as valuecalls).
const maxCallerDepth = 3

// maxCallerForms is how many functions of callers are kept with their forms; the walk starts again from an empty set past it.
const maxCallerForms = 256

type paramKey struct {
	fn    ir.FuncID
	index int
}

type readFunc struct {
	fn   *ir.Func
	form *irssa.Form
}

// callerWalk follows the parameter of a helper to what its callers pass there.
type callerWalk struct {
	src   ir.Source
	res   *Result
	sited ir.Sited
	forms map[ir.FuncID]*readFunc
	ix    *ir.ClosureIndex
}

// closureIndex is the index of the closures of the program (Rust), made when the first closure is met.
func (w *callerWalk) closureIndex() *ir.ClosureIndex {
	if w.ix == nil {
		w.ix = ir.NewClosureIndexOf(w.src)
	}
	return w.ix
}

// funcOf is the function that the caller at c is, with its form: made once while it is kept.
func (w *callerWalk) funcOf(c ir.CallSite) *readFunc {
	ref := ir.Ref{ID: w.src.Head(int(c.Seq)).ID, Seq: int(c.Seq)}
	w.res.callbackFormReads++
	if rf, ok := w.forms[ref.ID]; ok {
		return rf
	}
	if len(w.forms) >= maxCallerForms {
		w.forms = map[ir.FuncID]*readFunc{}
	}
	w.res.callbackFormBuilds++
	fn := ir.Must(w.src, ref)
	form, _ := irssa.Build(fn)
	rf := &readFunc{fn: fn, form: form}
	w.forms[ref.ID] = rf
	return rf
}

// functionsOf is the functions that the value r of fn may be, with the reason when some function was not found: a function that
// the value is made of (functionOf), or, when the value is a parameter of fn (a helper that registers what it is given), the functions that
// the callers of fn pass there, to maxCallerDepth. A parameter that the walk is already on is not followed again (a helper that calls
// itself): the other callers tell what it is given.
func (w *callerWalk) functionsOf(fn *ir.Func, form *irssa.Form, r irssa.Ref, depth int, seen map[paramKey]bool) ([]ir.FuncID, string) {
	if r.Kind == irssa.RefInstr {
		if in := form.Func.Blocks[r.Block].Instrs[r.Index]; in.Op == ir.OpParam {
			key := paramKey{fn.ID, in.Index}
			if seen[key] {
				return nil, ""
			}
			if depth >= maxCallerDepth {
				return nil, "a parameter of the function that makes the call, and its callers are more than three calls up (a helper that registers what it is given)"
			}
			if w.sited == nil {
				w.sited = ir.SitedOf(w.src)
			}
			callers := w.sited.CallersOf(fn.ID)
			if len(callers) == 0 {
				return nil, "a parameter of the function that makes the call, which no call in the program reaches (a helper that registers what it is given)"
			}
			seen[key] = true
			defer delete(seen, key)
			var ids []ir.FuncID
			why := ""
			for _, c := range callers {
				rf := w.funcOf(c)
				cin := rf.fn.Blocks[c.Block].Instrs[c.Instr]
				off := 0
				if cin.Callee != nil && cin.Callee.Kind == ir.CalleeValue {
					off = 1 // the first operand of a call of a function value is the value
				}
				if rf.form == nil || in.Index+off >= len(cin.Args) {
					why = "a parameter of the function that makes the call, and a caller of it is not known (a helper that registers what it is given)"
					continue
				}
				got, wy := w.functionsOf(rf.fn, rf.form, rf.form.Operand(int(c.Block), int(c.Instr), in.Index+off), depth+1, seen)
				ids = append(ids, got...)
				if wy != "" && why == "" {
					why = wy
				}
			}
			return ids, why
		}
	}
	id, why := functionOf(form, r, 0, w.closureIndex)
	if id == "" {
		return nil, why
	}
	return []ir.FuncID{id}, ""
}
