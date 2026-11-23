package taint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
)

// A function that has no body in the program and no model (a function of the standard library, or of a dependency
// that is not read) may call a function value that it is given and use what it returns. The default for such a
// call is that what goes in comes out (unmodeled_library_call), and a closure that is given to it carries nothing
// of what it captured into that: its captured values are applied where it is made, as if it were called, and what
// it returns is left out (applyMade says so). So `re.ReplaceAllStringFunc(s, func(string) string { return v })`
// has a result without v, with nothing said about it.
//
// For such a call the rows of calls that stand for it are made here, as for a package whose body is not read
// (bodyreplaced.go): each function value that is given, and that the program has a body for, is taken to be called,
// and what it returns is taken to reach the result of the call (assumed_call, the function named in the note).
// A function that has a model, or a body, is not affected: its rows, or its body, say what it does.

// noModelCallee reports whether the call is of a function that has no body in the program and no model, that is a
// call that stays unmodeled: a function without a body (not a foreign function, reflection, a body that could not
// be read, or a package whose rows are declared to stand for its body: those say it their own way), or a method of
// an interface that no implementation of the program is known for.
func (e *Engine) noModelCallee(ci *sg.Instr) bool {
	if e.rules == nil || ci.Callee == nil || ci.Callee.FFI {
		return false
	}
	sym := ci.Callee.Symbol
	if sym == "" || builtinName(ci) != "" || e.rules.HasModel(sym) || len(e.rules.CallSources[sym]) > 0 {
		return false
	}
	if ci.StaticID == "" {
		return ci.Callee.Kind == ir.CalleeDynamic && len(ci.Candidates) == 0
	}
	if ci.Static != nil {
		return false
	}
	ids := []string{string(ci.StaticID)}
	if kind, _ := externalKind(ids, sym); kind != KindUnmodeledLibraryCall {
		return false
	}
	return e.skippedBody(ids, sym) == nil && e.replacedUnit(ci) == ""
}

// noModelRows are the rows of calls that stand for the function values given to a function that has no body and no
// model: each is called with the other arguments (then the receiver), and what it returns is the result.
func (e *Engine) noModelRows(ci *sg.Instr) []callRowRef {
	if len(ci.Dsts) == 0 || !e.noModelCallee(ci) {
		return nil
	}
	recv, args := ci.Recv(), ci.CallArgs()
	var out []callRowRef
	for i, a := range args {
		cs, ok := e.callbacksOf(a)
		if !ok {
			continue // not told which function it is: what goes in comes out already (the default of the call)
		}
		var with []string
		for j := range args {
			if j != i {
				with = append(with, fmt.Sprintf("arg%d", j))
			}
		}
		if recv != nil {
			with = append(with, "recv")
		}
		ids := make([]string, len(cs))
		for k, c := range cs {
			ids[k] = c.ID
		}
		sort.Strings(ids)
		row := model.CallRow{
			Fn:   fmt.Sprintf("arg%d", i),
			With: with,
			Ret:  "result",
			Assumed: "assumed_call: " + ci.Callee.Symbol + " has no body and no model (or no implementation is known), and is given the function " + strings.Join(ids, " or ") +
				" as its argument " + fmt.Sprint(i) + ": it is taken to be called by it, with what the other operands hold, and what it returns is taken to reach the result of the call",
		}
		out = append(out, callRowRef{&model.Propagator{Symbol: ci.Callee.Symbol, Calls: []model.CallRow{row}}, 0})
	}
	return out
}

// noModelCallbacks applies the rows of noModelRows at the call.
func (s *state) noModelCallbacks(cc *callCtx) {
	for _, r := range s.eng.noModelRows(cc.in) {
		s.applyCallRow(cc, r.p, 0, modelRef(cc.sym, "propagator", model.Provenance{Origin: model.OriginBuiltin}))
	}
}
