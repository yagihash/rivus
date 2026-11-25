package taint

import (
	"fmt"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
)

// A package that a rule file declares (model.BodyReplaced) is not read: its functions have no body in the program.
// What a function of it does with a function that it is given, or with a value of a type of the project that it is
// given (a call back into the code of the project) is then said by its model, in a row of calls. Where the model has
// none (the function has no row, or its rows say nothing of calls), the call back is not left out in silence: a
// function value that is given is taken to be called by the function, with what the other operands hold, and its
// result is the result of the call (assumed_call, the same as the flows that stand for a row of calls when the
// function given is not known); a value of a type of the project that is given as an interface is told in a note.

// replacedUnit is the package of the callee of the call when it is one whose body the rules declare replaced, or "".
func (e *Engine) replacedUnit(ci *sg.Instr) string {
	if e.rules == nil || !e.rules.HasDeclared() || ci.Callee == nil {
		return ""
	}
	id := string(ci.StaticID)
	if id == "" {
		id = ci.Callee.Symbol
	}
	if id == "" {
		return ""
	}
	if u := UnitOf(id); e.rules.ReplacedBody(u) {
		return u
	}
	return ""
}

// replacedRows are the rows of calls that stand for the function values given to a function of a declared package
// whose model says nothing of calls: each function value is called with the other operands, and what it returns is
// the result. The positions count the receiver apart, as the rows of a model do.
func (e *Engine) replacedRows(ci *sg.Instr) []callRowRef {
	unit := e.replacedUnit(ci)
	if unit == "" || e.rules.Calls[ci.Callee.Symbol] {
		return nil
	}
	recv, args := ci.Recv(), ci.CallArgs()
	var out []callRowRef
	for i, a := range args {
		if !isFuncOperand(e, a) {
			continue
		}
		var with []string
		if recv != nil {
			with = append(with, "recv")
		}
		for j := range args {
			if j != i {
				with = append(with, fmt.Sprintf("arg%d", j))
			}
		}
		row := model.CallRow{
			Fn:   fmt.Sprintf("arg%d", i),
			With: with,
			Assumed: "assumed_call: the function given to " + ci.Callee.Symbol + " is taken to be called by it, with what the other operands hold, and what it returns is the result of the call: the body of the package " + unit +
				" is not read (the model stands for it) and the model has no row of calls for the function",
		}
		if len(ci.Dsts) > 0 {
			row.Ret = "result"
		}
		out = append(out, callRowRef{&model.Propagator{Symbol: ci.Callee.Symbol, Calls: []model.CallRow{row}}, 0})
	}
	return out
}

func isFuncOperand(e *Engine, v *sg.Value) bool {
	if v == nil {
		return false
	}
	if v.Class == ir.ClassFunc || v.Kind == sg.KFunc {
		return true
	}
	_, ok := e.callbacksOf(v)
	return ok
}

// replacedCallbacks applies, at a call of a function of a declared package, what the call may call back, when the
// model does not say it (see above).
func (s *state) replacedCallbacks(cc *callCtx) {
	ci := cc.in
	e := s.eng
	unit := e.replacedUnit(ci)
	if unit == "" || e.rules.Calls[cc.sym] {
		return
	}
	for _, r := range e.replacedRows(ci) {
		s.applyCallRow(cc, r.p, 0, modelRef(cc.sym, model.RolePropagator, model.Provenance{Origin: model.OriginBuiltin}))
	}
	ops := cc.argv()
	for i, v := range ops {
		impl, t := e.projectImpl(v)
		if t == "" {
			continue
		}
		fns := e.implMethods(t)
		if len(fns) == 0 || len(fns) > maxImplMethods {
			s.newNote(ci, KindUnresolvedCallee, "a value of "+t+", a type of the project, is given to "+cc.sym+
				" as an interface: the body of the package "+unit+" is not read and the model has no row of calls for the function, so the methods that the package may call on it are not followed", cc.sym)
			continue
		}
		// every method of the type may be the one that the package calls: each is called with what the other operands hold
		note := s.newNote(ci, KindAssumedCall, "assumed_call: a value of "+t+", a type of the project, is given to "+cc.sym+
			" as an interface: the body of the package "+unit+" is not read and the model has no row of calls for the function, so each method of the type is taken to be called, with what the other operands hold, and what it returns is the result of the call", cc.sym)
		for _, fn := range fns {
			args := make([]*sg.Value, len(fn.Params))
			args[0] = impl
			next := 0
			for k := 1; k < len(args); k++ {
				for next < len(ops) && next == i {
					next++
				}
				if next < len(ops) {
					args[k] = ops[next]
					next++
				} else {
					args[k] = &sg.Value{Kind: sg.KUndef}
				}
			}
			rd := &redirect{}
			if len(cc.roots) > 0 {
				rd.ret = []Loc{{cc.roots[0], ""}}
			}
			x := &callCtx{in: ci, args: args, roots: cc.roots, sym: cc.sym, rd: rd}
			s.applySummary(x, fn, nil, []*Uncertainty{note}, nil)
		}
	}
}

// maxImplMethods is how many methods of a type of the project are taken to be called by a package that is not read.
// A type with more is told in a note and not followed.
const maxImplMethods = 8

// implMethods are the methods of the project of the type (a type with a pointer receiver, or without), by ID.
func (e *Engine) implMethods(t string) []*sg.Func {
	t = strings.TrimLeft(t, "*")
	e.methodsOnce.Do(func() {
		e.methodsByType = map[string][]*sg.Func{}
		for _, fn := range e.prog.Funcs {
			if fn.Dep || !fn.Recv || len(fn.Params) == 0 || strings.Contains(fn.ID, "$") {
				continue
			}
			rt := strings.TrimLeft(string(fn.Params[0].Type), "*")
			e.methodsByType[rt] = append(e.methodsByType[rt], fn)
		}
	})
	return e.methodsByType[t]
}

// implFuncs are the methods that the call may call back on the values it is given (for the order of the analysis).
func (e *Engine) implFuncs(ci *sg.Instr) []*sg.Func {
	if e.replacedUnit(ci) == "" || e.rules.Calls[ci.Callee.Symbol] {
		return nil
	}
	var out []*sg.Func
	for _, v := range append([]*sg.Value{ci.Recv()}, ci.CallArgs()...) {
		if _, t := e.projectImpl(v); t != "" {
			if fns := e.implMethods(t); len(fns) <= maxImplMethods {
				out = append(out, fns...)
			}
		}
	}
	return out
}

// projectImpl is the value that was made an interface (make_interface) and its type, when the type is one of the
// project's own, or "".
func (e *Engine) projectImpl(v *sg.Value) (*sg.Value, string) {
	for i := 0; i < 4 && v != nil && v.Kind == sg.KResult && v.Instr != nil; i++ {
		in := v.Instr
		if in.Op != ir.OpConvert || in.IR == nil || len(in.Args) != 1 {
			return nil, ""
		}
		switch in.IR.Operator {
		case "make_interface":
			t := string(in.Args[0].Type)
			if !e.isProjectType(t) {
				return nil, ""
			}
			return in.Args[0], t
		case "change_interface":
			v = in.Args[0]
		default:
			return nil, ""
		}
	}
	return nil, ""
}

// isProjectType: the type is declared in a package that has a function of the code of the project (Go: the type
// "*example.com/m/pkg.T" is of the package "example.com/m/pkg").
func (e *Engine) isProjectType(t string) bool {
	t = strings.TrimLeft(t, "*[]&")
	if !strings.Contains(t, ".") {
		return false
	}
	e.projectOnce.Do(func() {
		e.projectUnits = map[string]bool{}
		for _, fn := range e.prog.Funcs {
			if !fn.Dep {
				e.projectUnits[UnitOf(fn.ID)] = true
			}
		}
	})
	return e.projectUnits[UnitOf(t)]
}
