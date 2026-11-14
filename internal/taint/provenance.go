package taint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/yagihash/rivus/internal/frontend/gossa"
)

// Limits of the tracing. When one is reached the tracing stops there and
// says so (depth_limit), instead of guessing.
const (
	maxProvDepth   = 6
	maxProvNodes   = 300
	maxProvCallers = 8
	maxProvResults = 20
)

// provenanceOf traces where the arguments that decide a condition come from.
func (e *Engine) provenanceOf(g Guard) *Provenance {
	t := &tracer{e: e, res: &Provenance{Resolved: []ResolvedValue{}, Unresolved: []UnresolvedValue{}}, keys: map[string]bool{}}
	var names []string
	for _, p := range g.Params {
		if p < len(g.fn.Params) {
			names = append(names, fmt.Sprintf("%s (param %d)", g.fn.Params[p].Name(), p))
		}
	}
	t.res.Argument = strings.Join(names, ", ")
	if g.Call != nil {
		for _, a := range g.Call.Args {
			text, _ := e.prog.CallArgText(g.Call.pos, a.ArgIndex)
			t.walk(a.value, g.Call.fn, []string{text}, g.Call.pos, 0)
		}
	} else {
		for _, p := range g.Params {
			if p < len(g.fn.Params) {
				t.walk(g.fn.Params[p], g.fn, nil, token.NoPos, 0)
			}
		}
	}
	return t.res
}

type tracer struct {
	e      *Engine
	res    *Provenance
	nodes  int
	keys   map[string]bool
	inPath map[ssa.Value]bool
}

// Certainty of a resolved value.
const (
	CertaintyLiteral   = "literal"
	CertaintyZeroValue = "zero_value" // nothing sets it: the zero value of the type
)

func (t *tracer) resolved(val string, site token.Pos, certainty string) {
	k := "r|" + val + "|" + itoa(int(site)) + "|" + certainty
	if t.keys[k] || len(t.res.Resolved) >= maxProvResults {
		return
	}
	t.keys[k] = true
	t.res.Resolved = append(t.res.Resolved, ResolvedValue{Value: val, CallSite: t.e.prog.Position(site), Certainty: certainty})
}

// stop records a place where the tracing could not go on.
func (t *tracer) stop(kind, detail, symbol string, pos token.Pos, chain []string, candidates []string) {
	k := "u|" + kind + "|" + symbol + "|" + itoa(int(pos)) + "|" + detail
	if t.keys[k] || len(t.res.Unresolved) >= maxProvResults {
		return
	}
	t.keys[k] = true
	u := UnresolvedValue{
		TerminalKind:   kind,
		AttributableTo: attributable(kind),
		Chain:          append([]string(nil), chain...),
		Detail:         detail,
		Candidates:     candidates,
	}
	if symbol != "" && detail == "" {
		u.Detail = symbol
	}
	if pos.IsValid() {
		u.Position = t.e.prog.Position(pos)
		u.Snippet = t.e.prog.Snippet(pos)
	}
	t.res.Unresolved = append(t.res.Unresolved, u)
}

// walk follows the value back to where it is made. fn is the function the
// value lives in and site the call that handed it down.
func (t *tracer) walk(v ssa.Value, fn *ssa.Function, chain []string, site token.Pos, depth int) {
	if v == nil {
		return
	}
	t.nodes++
	if depth > maxProvDepth || t.nodes > maxProvNodes {
		t.stop(KindDepthLimit, "the tracing was cut at its depth or size limit", "", v.Pos(), chain, nil)
		return
	}
	if t.inPath == nil {
		t.inPath = map[ssa.Value]bool{}
	}
	if t.inPath[v] {
		return // a loop
	}
	t.inPath[v] = true
	defer delete(t.inPath, v)

	switch x := v.(type) {
	case *ssa.Const:
		if x.Value == nil {
			t.resolved("nil", site, CertaintyLiteral)
		} else {
			t.resolved(x.Value.ExactString(), site, CertaintyLiteral)
		}
	case *ssa.Phi:
		for _, e := range x.Edges {
			t.walk(e, fn, chain, site, depth)
		}
	case *ssa.ChangeType:
		t.walk(x.X, fn, chain, site, depth)
	case *ssa.Convert:
		t.walk(x.X, fn, chain, site, depth)
	case *ssa.MakeInterface:
		t.walk(x.X, fn, chain, site, depth)
	case *ssa.TypeAssert:
		t.walk(x.X, fn, chain, site, depth)
	case *ssa.BinOp:
		// A constant compared with the value is not where the value comes from.
		cmp := isComparison(x.Op)
		for _, o := range []ssa.Value{x.X, x.Y} {
			if _, isConst := o.(*ssa.Const); cmp && isConst {
				continue
			}
			t.walk(o, fn, chain, site, depth)
		}
	case *ssa.UnOp:
		if x.Op == token.MUL {
			t.walkAddr(x.X, fn, chain, site, depth)
			return
		}
		t.walk(x.X, fn, chain, site, depth)
	case *ssa.Field:
		t.walk(x.X, fn, append(chain, "."+fieldName(x.X.Type(), x.Field)), site, depth)
	case *ssa.Extract:
		if call, ok := x.Tuple.(*ssa.Call); ok {
			t.walkResult(call, x.Index, fn, chain, depth)
			return
		}
		t.stop(KindDepthLimit, "a value taken out of a "+types.TypeString(x.Tuple.Type(), nil), "", x.Pos(), chain, nil)
	case *ssa.Call:
		t.walkResult(x, 0, fn, chain, depth)
	case *ssa.Parameter:
		t.walkParam(x, fn, chain, depth)
	case *ssa.Global:
		t.stopGlobal(x, chain)
	case *ssa.Lookup, *ssa.Index:
		t.stop(KindContainerImprecision, "an element of a container: the elements are not told apart", "", v.Pos(), chain, nil)
	default:
		t.stop(KindDepthLimit, "not followed: "+fmt.Sprintf("%T", v), "", v.Pos(), chain, nil)
	}
}

func (t *tracer) stopGlobal(g *ssa.Global, chain []string) {
	t.stop(KindGlobalVar, "the value is the package variable "+g.String()+"; its direct writers are listed", g.String(), g.Pos(),
		chain, t.e.writersOf(g))
}

// walkParam goes from a parameter to the arguments of its callers.
func (t *tracer) walkParam(p *ssa.Parameter, fn *ssa.Function, chain []string, depth int) {
	idx := -1
	for i, q := range fn.Params {
		if q == p {
			idx = i
		}
	}
	chain = append(chain, fmt.Sprintf("%s (parameter of %s)", p.Name(), gossa.FuncID(fn)))
	sites := t.e.callersOf(fn)
	if len(sites) == 0 {
		if o, ok := fn.Object().(*types.Func); ok && ast.IsExported(o.Name()) {
			t.stop(KindExportedAPIParam, "the value comes from outside the analysed code, through the exported function "+gossa.FuncID(fn), "", p.Pos(), chain, nil)
		} else {
			t.stop(KindUnresolvedCallee, "no call of "+gossa.FuncID(fn)+" was found; it may be called through a function value", "", p.Pos(), chain, nil)
		}
		return
	}
	if len(sites) > maxProvCallers {
		t.stop(KindDepthLimit, fmt.Sprintf("%s has %d callers; only the first %d are followed", gossa.FuncID(fn), len(sites), maxProvCallers), "", p.Pos(), chain, nil)
		sites = sites[:maxProvCallers]
	}
	for _, ci := range sites {
		recv, args := gossa.CallArgs(ci.Common())
		argv := args
		if recv != nil {
			argv = append([]ssa.Value{recv}, args...)
		}
		if idx < 0 || idx >= len(argv) {
			continue
		}
		t.walk(argv[idx], ci.Parent(), chain, ci.Pos(), depth+1)
	}
}

// walkResult follows the result of a call into the callee, or stops at a
// function that is not analysed.
func (t *tracer) walkResult(call *ssa.Call, idx int, fn *ssa.Function, chain []string, depth int) {
	sym := gossa.CalleeSymbol(call.Common())
	chain = append(chain, orUnknown(sym)+"()")
	if t.e.rules.ConfigReads[sym] {
		t.stop(KindConfigRead, "the value is read from configuration by "+sym, sym, call.Pos(), chain, nil)
		return
	}
	var callees []*ssa.Function
	if f := call.Common().StaticCallee(); f != nil {
		callees = []*ssa.Function{f}
	} else if t.e.graph != nil {
		callees = t.e.graph.Callees(call)
	}
	var bodies []*ssa.Function
	for _, c := range callees {
		if t.e.hasBody(c) {
			bodies = append(bodies, c)
		}
	}
	switch {
	case len(callees) == 0 && call.Common().IsInvoke():
		t.stop(KindDynamicDispatch, "no implementation is known: a model for the interface method would resolve this", sym, call.Pos(), chain, nil)
	case len(callees) == 0:
		t.stop(KindUnresolvedCallee, "the called function is not known", sym, call.Pos(), chain, nil)
	case len(bodies) == 0:
		kind, detail := externalKind(callees, sym)
		t.stop(kind, detail, sym, call.Pos(), chain, nil)
	case len(bodies) > 4:
		var cands []string
		for _, b := range bodies {
			cands = append(cands, gossa.FuncID(b))
		}
		sort.Strings(cands)
		t.stop(KindDynamicDispatch, fmt.Sprintf("%d possible callees", len(bodies)), sym, call.Pos(), chain, cands)
	default:
		for _, b := range bodies {
			for _, blk := range b.Blocks {
				if len(blk.Instrs) == 0 {
					continue
				}
				if ret, ok := blk.Instrs[len(blk.Instrs)-1].(*ssa.Return); ok && idx < len(ret.Results) {
					t.walk(ret.Results[idx], b, chain, call.Pos(), depth+1)
				}
			}
		}
	}
}

// walkAddr follows a value that is read from memory: a variable, a field.
func (t *tracer) walkAddr(addr ssa.Value, fn *ssa.Function, chain []string, site token.Pos, depth int) {
	switch a := addr.(type) {
	case *ssa.Global:
		t.stopGlobal(a, chain)
	case *ssa.FieldAddr:
		t.walkObject(a.X, a.Field, fieldName(a.X.Type(), a.Field), fn, chain, site, depth)
	case *ssa.IndexAddr:
		t.stop(KindContainerImprecision, "an element of a container: the elements are not told apart", "", a.Pos(), chain, nil)
	case *ssa.Alloc:
		// A variable of the function: what is stored into it.
		stored := false
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if st, ok := in.(*ssa.Store); ok && st.Addr == addr {
					stored = true
					t.walk(st.Val, fn, chain, site, depth+1)
				}
			}
		}
		if !stored {
			t.stop(KindDepthLimit, "the variable is not assigned in the function that declares it", "", a.Pos(), chain, nil)
		}
	default:
		t.stop(KindDepthLimit, "a value read through a pointer that is not followed", "", addr.Pos(), chain, nil)
	}
}

// walkObject follows the field of an object back to where the field is set.
func (t *tracer) walkObject(base ssa.Value, field int, name string, fn *ssa.Function, chain []string, site token.Pos, depth int) {
	chain = append(chain, "."+name)
	switch b := base.(type) {
	case *ssa.Global:
		t.stop(KindGlobalVar, "the field "+name+" of the package variable "+b.String()+"; its direct writers are listed", b.String()+"."+name, b.Pos(),
			chain, t.e.writersOf(b))
	case *ssa.Alloc:
		set := false
		for _, blk := range fn.Blocks {
			for _, in := range blk.Instrs {
				st, ok := in.(*ssa.Store)
				if !ok {
					continue
				}
				if fa, ok := st.Addr.(*ssa.FieldAddr); ok && fa.X == base && fa.Field == field {
					set = true
					t.walk(st.Val, fn, chain, site, depth+1)
				}
			}
		}
		switch {
		case allocEscapes(b):
			t.stop(KindDepthLimit, "the address of the object is passed on: the field may be changed elsewhere", "", b.Pos(), chain, nil)
		case !set:
			// Nothing sets it and nobody else can: the zero value.
			t.resolved(zeroValue(b.Type().Underlying().(*types.Pointer).Elem().Underlying().(*types.Struct).Field(field).Type()), site, CertaintyZeroValue)
		}
	case *ssa.Parameter:
		// The object is handed in: look at what the callers pass.
		idx := -1
		for i, q := range fn.Params {
			if q == b {
				idx = i
			}
		}
		sites := t.e.callersOf(fn)
		if len(sites) == 0 {
			if o, ok := fn.Object().(*types.Func); ok && ast.IsExported(o.Name()) {
				t.stop(KindExportedAPIParam, "the object comes from outside the analysed code, through "+gossa.FuncID(fn), "", b.Pos(), chain, nil)
			} else {
				t.stop(KindUnresolvedCallee, "no call of "+gossa.FuncID(fn)+" was found", "", b.Pos(), chain, nil)
			}
			return
		}
		if len(sites) > maxProvCallers {
			sites = sites[:maxProvCallers]
		}
		for _, ci := range sites {
			recv, args := gossa.CallArgs(ci.Common())
			argv := args
			if recv != nil {
				argv = append([]ssa.Value{recv}, args...)
			}
			if idx >= 0 && idx < len(argv) {
				t.walkObject(argv[idx], field, name, ci.Parent(), chain[:len(chain)-1], ci.Pos(), depth+1)
			}
		}
	default:
		t.stop(KindDepthLimit, "the object that holds the field is reached through a pointer that is not followed", "", base.Pos(), chain, nil)
	}
}

// allocEscapes reports whether the address of a variable is used for
// anything but reading and writing its fields: then others may change it.
func allocEscapes(a *ssa.Alloc) bool {
	for _, r := range *a.Referrers() {
		switch r := r.(type) {
		case *ssa.DebugRef:
		case *ssa.FieldAddr:
			for _, rr := range *r.Referrers() {
				switch rr := rr.(type) {
				case *ssa.DebugRef:
				case *ssa.UnOp:
					if rr.Op != token.MUL {
						return true
					}
				case *ssa.Store:
					if rr.Addr != ssa.Value(r) {
						return true // the address itself is stored somewhere
					}
				default:
					return true
				}
			}
		case *ssa.Store:
			if r.Addr != ssa.Value(a) {
				return true
			}
		case *ssa.UnOp:
			if r.Op != token.MUL {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// zeroValue spells the zero value of a type.
func zeroValue(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsBoolean != 0:
			return "false"
		case u.Info()&types.IsString != 0:
			return `""`
		case u.Info()&types.IsNumeric != 0:
			return "0"
		}
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Interface, *types.Signature:
		return "nil"
	}
	return "zero value of " + types.TypeString(t, nil)
}

// callersOf lists the call instructions that may call the function.
func (e *Engine) callersOf(fn *ssa.Function) []ssa.CallInstruction {
	if e.callSites == nil {
		e.callSites = map[*ssa.Function][]ssa.CallInstruction{}
		for _, caller := range e.prog.Funcs {
			for _, b := range caller.Blocks {
				for _, in := range b.Instrs {
					ci, ok := in.(ssa.CallInstruction)
					if !ok {
						continue
					}
					var callees []*ssa.Function
					if f := ci.Common().StaticCallee(); f != nil {
						callees = []*ssa.Function{f}
					} else if e.graph != nil {
						callees = e.graph.Callees(ci)
					}
					for _, c := range callees {
						e.callSites[c] = append(e.callSites[c], ci)
					}
				}
			}
		}
	}
	return e.callSites[fn]
}
