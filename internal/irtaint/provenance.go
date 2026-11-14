package irtaint

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
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
			names = append(names, fmt.Sprintf("%s (param %d)", g.fn.Params[p].Name, p))
		}
	}
	t.res.Argument = strings.Join(names, ", ")
	if g.Call != nil {
		for _, a := range g.Call.Args {
			text, _ := e.texts.CallArg(g.Call.pos, a.ArgIndex)
			t.walk(a.value, g.Call.fn, []string{text}, g.Call.pos, 0)
		}
	} else {
		for _, p := range g.Params {
			if p < len(g.fn.Params) {
				t.walk(g.fn.Params[p], g.fn, nil, ir.Pos{}, 0)
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
	inPath map[*sg.Value]bool
}

// Certainty of a resolved value.
const (
	CertaintyLiteral   = "literal"
	CertaintyZeroValue = "zero_value" // nothing sets it: the zero value of the type
)

func (t *tracer) resolved(val string, site ir.Pos, certainty string) {
	k := "r|" + val + "|" + posKey(site) + "|" + certainty
	if t.keys[k] || len(t.res.Resolved) >= maxProvResults {
		return
	}
	t.keys[k] = true
	t.res.Resolved = append(t.res.Resolved, ResolvedValue{Value: val, CallSite: position(site), Certainty: certainty})
}

// stop records a place where the tracing could not go on.
func (t *tracer) stop(kind, detail, symbol string, pos ir.Pos, chain []string, candidates []string) {
	k := "u|" + kind + "|" + symbol + "|" + posKey(pos) + "|" + detail
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
	if !pos.IsZero() {
		u.Position = position(pos)
		u.Snippet = t.e.texts.Snippet(pos)
	}
	t.res.Unresolved = append(t.res.Unresolved, u)
}

// constText spells a literal as it is written in a result.
func constText(v *sg.Value) string {
	if v.Const == nil {
		return zeroText(v.Class, v.Type)
	}
	switch v.Const.Kind {
	case ir.ConstString:
		return strconv.Quote(v.Const.Value)
	case ir.ConstNil:
		return "nil"
	}
	return v.Const.Value
}

// zeroText spells the zero value of a type.
func zeroText(c ir.TypeClass, t ir.TypeID) string {
	switch c {
	case ir.ClassBool:
		return "false"
	case ir.ClassString:
		return `""`
	case ir.ClassNumber:
		return "0"
	case ir.ClassPointer, ir.ClassContainer, ir.ClassInterface, ir.ClassFunc:
		return "nil"
	}
	return "zero value of " + string(t)
}

// tupleText spells the type of a tuple the way go/types does.
func tupleText(in *sg.Instr) string {
	var ts []string
	for _, d := range in.Dsts {
		ts = append(ts, string(d.Type))
	}
	return "(" + strings.Join(ts, ", ") + ")"
}

// walk follows the value back to where it is made. fn is the function the
// value lives in and site the call that handed it down.
func (t *tracer) walk(v *sg.Value, fn *sg.Func, chain []string, site ir.Pos, depth int) {
	if v == nil {
		return
	}
	t.nodes++
	if depth > maxProvDepth || t.nodes > maxProvNodes {
		t.stop(KindDepthLimit, "the tracing was cut at its depth or size limit", "", v.Pos, chain, nil)
		return
	}
	if t.inPath == nil {
		t.inPath = map[*sg.Value]bool{}
	}
	if t.inPath[v] {
		return // a loop
	}
	t.inPath[v] = true
	defer delete(t.inPath, v)

	switch v.Kind {
	case sg.KConst:
		t.resolved(constText(v), site, CertaintyLiteral)
		return
	case sg.KPhi:
		for _, e := range v.Instr.Args {
			t.walk(e, fn, chain, site, depth)
		}
		return
	case sg.KParam:
		t.walkParam(v, fn, chain, depth)
		return
	case sg.KGlobal:
		t.stopGlobal(v, chain)
		return
	case sg.KResult:
	default:
		t.stop(KindDepthLimit, "not followed: "+notFollowed(v), "", v.Pos, chain, nil)
		return
	}

	in := v.Instr
	switch in.Op {
	case ir.OpConvert:
		switch in.IR.Operator {
		case "convert", "make_interface":
			t.walk(in.Args[0], fn, chain, site, depth)
		default:
			t.stop(KindDepthLimit, "not followed: "+notFollowed(v), "", v.Pos, chain, nil)
		}
	case ir.OpTypeAssert:
		if len(in.Dsts) == 2 {
			t.stop(KindDepthLimit, "a value taken out of a "+tupleText(in), "", v.Pos, chain, nil)
			return
		}
		t.walk(in.Args[0], fn, chain, site, depth)
	case ir.OpBinop:
		// A constant compared with the value is not where the value comes from.
		cmp := isComparison(in.IR.Operator)
		for _, o := range in.Args {
			if cmp && o.Kind == sg.KConst {
				continue
			}
			t.walk(o, fn, chain, site, depth)
		}
	case ir.OpUnop:
		t.walk(in.Args[0], fn, chain, site, depth)
	case ir.OpLoad:
		t.walkAddr(in.Args[0], fn, chain, site, depth)
	case ir.OpGlobalLoad:
		t.stopGlobal(t.e.prog.Global(in.IR.Global), chain)
	case ir.OpFieldLoad:
		if in.IR.Folded {
			t.walkObject(in.Args[0], in.IR.Field, v, fn, chain, site, depth)
			return
		}
		t.walk(in.Args[0], fn, append(chain, "."+in.IR.Field), site, depth)
	case ir.OpIndexLoad:
		pos := v.Pos
		if in.IR.Folded && !in.AddrPos.IsZero() {
			pos = in.AddrPos
		}
		t.stop(KindContainerImprecision, "an element of a container: the elements are not told apart", "", pos, chain, nil)
	case ir.OpCall:
		if len(in.Dsts) > 1 || v == in.Dsts[0] {
			t.walkResult(in, v.Index, fn, chain, depth)
			return
		}
		t.stop(KindDepthLimit, "not followed: "+notFollowed(v), "", v.Pos, chain, nil)
	default:
		t.stop(KindDepthLimit, "not followed: "+notFollowed(v), "", v.Pos, chain, nil)
	}
}

// notFollowed names the kind of a value that the tracing does not go
// through, as the first version of the message named it.
func notFollowed(v *sg.Value) string {
	switch v.Kind {
	case sg.KFreeVar:
		return "*ssa.FreeVar"
	case sg.KFunc:
		return "*ssa.Function"
	}
	if v.Instr == nil {
		return "value"
	}
	switch v.Instr.Op {
	case ir.OpAlloc:
		return "*ssa.Alloc"
	case ir.OpConvert:
		switch v.Instr.IR.Operator {
		case "change_interface":
			return "*ssa.ChangeInterface"
		case "slice_to_array_pointer":
			return "*ssa.SliceToArrayPointer"
		}
		return "*ssa.Convert"
	case ir.OpCopy:
		return "*ssa.Slice"
	case ir.OpAddrOf:
		return "*ssa.FieldAddr"
	case ir.OpMakeClosure:
		return "*ssa.MakeClosure"
	case ir.OpCall:
		return "*ssa.UnOp"
	}
	return "*ssa." + string(v.Instr.Op)
}

func (t *tracer) stopGlobal(g *sg.Value, chain []string) {
	t.stop(KindGlobalVar, "the value is the package variable "+g.Name+"; its direct writers are listed", g.Name, g.Pos,
		chain, t.e.writersOf(g))
}

// walkParam goes from a parameter to the arguments of its callers.
func (t *tracer) walkParam(p *sg.Value, fn *sg.Func, chain []string, depth int) {
	idx := p.Param
	chain = append(chain, fmt.Sprintf("%s (parameter of %s)", p.Name, fn.ID))
	sites := t.e.callersOf(fn)
	if len(sites) == 0 {
		if fn.Exported {
			t.stop(KindExportedAPIParam, "the value comes from outside the analysed code, through the exported function "+fn.ID, "", p.Pos, chain, nil)
		} else {
			t.stop(KindUnresolvedCallee, "no call of "+fn.ID+" was found; it may be called through a function value", "", p.Pos, chain, nil)
		}
		return
	}
	if len(sites) > maxProvCallers {
		t.stop(KindDepthLimit, fmt.Sprintf("%s has %d callers; only the first %d are followed", fn.ID, len(sites), maxProvCallers), "", p.Pos, chain, nil)
		sites = sites[:maxProvCallers]
	}
	for _, ci := range sites {
		argv := callArgv(ci)
		if idx < 0 || idx >= len(argv) {
			continue
		}
		t.walk(argv[idx], ci.Block.Func, chain, ci.Pos, depth+1)
	}
}

// callArgv lists the operands of a call in parameter order (receiver first).
func callArgv(ci *sg.Instr) []*sg.Value {
	var argv []*sg.Value
	if r := ci.Recv(); r != nil {
		argv = append(argv, r)
	}
	return append(argv, ci.CallArgs()...)
}

// walkResult follows the result of a call into the callee, or stops at a
// function that is not analysed.
func (t *tracer) walkResult(call *sg.Instr, idx int, fn *sg.Func, chain []string, depth int) {
	sym := call.Callee.Symbol
	if builtinName(call) != "" {
		sym = ""
	}
	chain = append(chain, orUnknown(sym)+"()")
	if t.e.rules.ConfigReads[sym] {
		t.stop(KindConfigRead, "the value is read from configuration by "+sym, sym, call.Pos, chain, nil)
		return
	}
	var callees []callee
	if call.StaticID != "" {
		callees = []callee{{string(call.StaticID), call.Static}}
	} else {
		for _, id := range call.Candidates {
			callees = append(callees, callee{string(id), t.e.prog.ByID[string(id)]})
		}
	}
	var bodies []*sg.Func
	for _, c := range callees {
		if c.fn != nil {
			bodies = append(bodies, c.fn)
		}
	}
	invoke := call.Callee.Kind == ir.CalleeDynamic
	switch {
	case len(callees) == 0 && invoke:
		t.stop(KindDynamicDispatch, "no implementation is known: a model for the interface method would resolve this", sym, call.Pos, chain, nil)
	case len(callees) == 0:
		t.stop(KindUnresolvedCallee, "the called function is not known", sym, call.Pos, chain, nil)
	case len(bodies) == 0:
		ids := make([]string, len(callees))
		for i, c := range callees {
			ids[i] = c.id
		}
		kind, detail := externalKind(ids, sym)
		t.stop(kind, detail, sym, call.Pos, chain, nil)
	case len(bodies) > 4:
		var cands []string
		for _, b := range bodies {
			cands = append(cands, b.ID)
		}
		sort.Strings(cands)
		t.stop(KindDynamicDispatch, fmt.Sprintf("%d possible callees", len(bodies)), sym, call.Pos, chain, cands)
	default:
		for _, b := range bodies {
			for _, blk := range b.Blocks {
				if ret := blk.Terminator(); ret != nil && ret.Op == sg.OpReturn && idx < len(ret.Args) {
					t.walk(ret.Args[idx], b, chain, call.Pos, depth+1)
				}
			}
		}
	}
}

// walkAddr follows a value that is read from memory: a variable, a field.
func (t *tracer) walkAddr(addr *sg.Value, fn *sg.Func, chain []string, site ir.Pos, depth int) {
	switch {
	case addr.Kind == sg.KGlobal:
		t.stopGlobal(addr, chain)
	case addr.Instr != nil && addr.Instr.Op == ir.OpAddrOf && len(addr.Instr.Args) == 1 && len(addr.Instr.IR.Path) > 0:
		a := addr.Instr
		switch a.IR.Path[0].Kind {
		case ir.StepField:
			t.walkObject(a.Args[0], a.IR.Path[0].Field, nil, fn, chain, site, depth)
		default:
			t.stop(KindContainerImprecision, "an element of a container: the elements are not told apart", "", a.Pos, chain, nil)
		}
	case addr.Instr != nil && addr.Instr.Op == ir.OpAlloc:
		// A variable of the function: what is stored into it.
		stored := false
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpStore && in.Args[0] == addr {
					stored = true
					t.walk(in.Args[1], fn, chain, site, depth+1)
				}
			}
		}
		if !stored {
			t.stop(KindDepthLimit, "the variable is not assigned in the function that declares it", "", addr.Pos, chain, nil)
		}
	default:
		t.stop(KindDepthLimit, "a value read through a pointer that is not followed", "", addr.Pos, chain, nil)
	}
}

// walkObject follows the field of an object back to where the field is set.
// read is the value the field was read into: its type is the type of the
// field.
func (t *tracer) walkObject(base *sg.Value, field string, read *sg.Value, fn *sg.Func, chain []string, site ir.Pos, depth int) {
	chain = append(chain, "."+field)
	switch {
	case base.Kind == sg.KGlobal:
		t.stop(KindGlobalVar, "the field "+field+" of the package variable "+base.Name+"; its direct writers are listed", base.Name+"."+field, base.Pos,
			chain, t.e.writersOf(base))
	case base.Instr != nil && base.Instr.Op == ir.OpAlloc:
		set := false
		for _, blk := range fn.Blocks {
			for _, in := range blk.Instrs {
				switch {
				case in.Op == ir.OpFieldStore && in.Args[0] == base && in.IR.Field == field:
					set = true
					t.walk(in.Args[1], fn, chain, site, depth+1)
				case in.Op == ir.OpStore && in.Args[0].Instr != nil && in.Args[0].Instr.Op == ir.OpAddrOf:
					fa := in.Args[0].Instr
					if len(fa.Args) == 1 && fa.Args[0] == base && len(fa.IR.Path) == 1 && fa.IR.Path[0].Kind == ir.StepField && fa.IR.Path[0].Field == field {
						set = true
						t.walk(in.Args[1], fn, chain, site, depth+1)
					}
				}
			}
		}
		switch {
		case allocEscapes(fn, base):
			t.stop(KindDepthLimit, "the address of the object is passed on: the field may be changed elsewhere", "", base.Pos, chain, nil)
		case !set:
			// Nothing sets it and nobody else can: the zero value.
			if read != nil {
				t.resolved(zeroText(read.Class, read.Type), site, CertaintyZeroValue)
			}
		}
	case base.Kind == sg.KParam:
		// The object is handed in: look at what the callers pass.
		idx := base.Param
		sites := t.e.callersOf(fn)
		if len(sites) == 0 {
			if fn.Exported {
				t.stop(KindExportedAPIParam, "the object comes from outside the analysed code, through "+fn.ID, "", base.Pos, chain, nil)
			} else {
				t.stop(KindUnresolvedCallee, "no call of "+fn.ID+" was found", "", base.Pos, chain, nil)
			}
			return
		}
		if len(sites) > maxProvCallers {
			sites = sites[:maxProvCallers]
		}
		for _, ci := range sites {
			argv := callArgv(ci)
			if idx >= 0 && idx < len(argv) {
				t.walkObject(argv[idx], field, read, ci.Block.Func, chain[:len(chain)-1], ci.Pos, depth+1)
			}
		}
	default:
		t.stop(KindDepthLimit, "the object that holds the field is reached through a pointer that is not followed", "", base.Pos, chain, nil)
	}
}

// allocEscapes reports whether the address of a variable is used for
// anything but reading and writing its fields: then others may change it.
func allocEscapes(fn *sg.Func, a *sg.Value) bool {
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			for k, arg := range in.Args {
				if arg != a {
					continue
				}
				switch {
				case in.Op == ir.OpFieldLoad && k == 0, in.Op == ir.OpFieldStore && k == 0:
				case in.Op == ir.OpLoad && k == 0, in.Op == ir.OpStore && k == 0:
				case in.Op == ir.OpAddrOf && len(in.IR.Path) == 1 && in.IR.Path[0].Kind == ir.StepField && k == 0:
					// A field address: it must only be read or written.
					if addrEscapes(fn, in.Dsts[0]) {
						return true
					}
				default:
					return true
				}
			}
		}
	}
	return false
}

// addrEscapes reports whether an address is used for anything but a load or
// a store through it.
func addrEscapes(fn *sg.Func, v *sg.Value) bool {
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			for k, arg := range in.Args {
				if arg != v {
					continue
				}
				if !((in.Op == ir.OpLoad || in.Op == ir.OpStore) && k == 0) {
					return true
				}
			}
		}
	}
	return false
}

// callersOf lists the call instructions that may call the function.
func (e *Engine) callersOf(fn *sg.Func) []*sg.Instr {
	if e.callSites == nil {
		e.callSites = map[*sg.Func][]*sg.Instr{}
		for _, caller := range e.prog.Funcs {
			for _, b := range caller.Blocks {
				for _, in := range b.Instrs {
					if in.Op != ir.OpCall {
						continue
					}
					var callees []*sg.Func
					if in.StaticID != "" {
						if in.Static != nil {
							callees = []*sg.Func{in.Static}
						}
					} else {
						callees = in.CandidateFns
					}
					for _, c := range callees {
						e.callSites[c] = append(e.callSites[c], in)
					}
				}
			}
		}
	}
	return e.callSites[fn]
}
