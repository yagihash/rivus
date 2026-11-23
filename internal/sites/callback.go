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
	ir.WalkProject(irsrc, func(_ ir.Ref, fn *ir.Func) bool {
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
					id, why := functionOf(form, form.Operand(bi, ii, at), 0)
					if id == "" {
						res.noteCallback(IRInstr{fn.ID, bi, ii}, in.Callee.Symbol, *src.Operand, why)
						continue
					}
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
		return true
	})
	return out
}

// functionOf follows a value to the function that makes it, through copies and conversions that keep it whole.
func functionOf(form *irssa.Form, r irssa.Ref, depth int) (ir.FuncID, string) {
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
		return functionOf(form, form.Operand(r.Block, r.Index, 0), depth+1)
	case ir.OpParam:
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
