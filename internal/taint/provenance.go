package taint

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

type objKey struct {
	base  *sg.Value
	field string
}

type tracer struct {
	e      *Engine
	res    *Provenance
	nodes  int
	keys   map[string]bool
	inPath map[*sg.Value]int
	// opDepth is how many operations are being worked out around the
	// current point; a loop that is cut with a greater depth than it was
	// entered at goes through an operation.
	opDepth int
	// objPath are the (object, field) pairs being followed, to cut a loop
	// of calls that hand the same object on.
	objPath map[objKey]int
	// frame, when set, collects the values the tracing of an operand of an
	// operation reaches, instead of reporting them: they are not the value
	// of the operation.
	frame *frame
	// ctx are the calls whose result is being followed into the callee: a
	// parameter of such a callee is the argument of that call, not of every
	// caller.
	ctx []provCall
}

type provCall struct {
	call   *sg.Instr
	callee *sg.Func
}

type frame struct {
	vals   []capVal
	failed bool
}

type capVal struct {
	c    *provVal
	cert string
	site ir.Pos
}

// Certainty of a resolved value.
const (
	CertaintyLiteral   = "literal"
	CertaintyZeroValue = "zero_value" // nothing sets it: the zero value of the type
)

func (t *tracer) resolved(val string, site ir.Pos, certainty string, c *provVal) {
	if t.frame != nil {
		t.frame.vals = append(t.frame.vals, capVal{c, certainty, site})
		return
	}
	k := "r|" + val + "|" + posKey(site) + "|" + certainty
	if t.keys[k] || len(t.res.Resolved) >= maxProvResults {
		return
	}
	t.keys[k] = true
	t.res.Resolved = append(t.res.Resolved, ResolvedValue{Value: val, CallSite: position(site), Certainty: certainty})
}

// stop records a place where the tracing could not go on.
func (t *tracer) stop(kind, detail, symbol string, pos ir.Pos, chain []string, candidates []string) {
	if t.frame != nil {
		t.frame.failed = true
	}
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
		t.inPath = map[*sg.Value]int{}
	}
	if d, ok := t.inPath[v]; ok {
		t.cutLoop(d, v.Pos, chain)
		return
	}
	t.inPath[v] = t.opDepth
	defer delete(t.inPath, v)

	switch v.Kind {
	case sg.KConst:
		var cp *provVal
		if c, ok := provConst(v); ok {
			cp = &c
		}
		t.resolved(constText(v), site, CertaintyLiteral, cp)
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
		case "make_interface":
			t.walk(in.Args[0], fn, chain, site, depth)
		case "convert":
			t.derived(v, in.Args, "a conversion", fn, chain, site, depth, func(a []provVal) (provVal, bool) {
				return foldConvert(a[0], v)
			})
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
		if len(in.Args) != 2 {
			t.stop(KindDepthLimit, "not followed: "+notFollowed(v), "", v.Pos, chain, nil)
			return
		}
		t.derived(v, in.Args, "the operation "+in.IR.Operator, fn, chain, site, depth, func(a []provVal) (provVal, bool) {
			return foldBinop(in.IR.Operator, a[0], a[1], v)
		})
	case ir.OpUnop:
		t.derived(v, in.Args, "the operation "+in.IR.Operator, fn, chain, site, depth, func(a []provVal) (provVal, bool) {
			return foldUnop(in.IR.Operator, a[0], v)
		})
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

// cutLoop is a value reached again while it is being followed. A loop of
// copies adds no value, so it is cut quietly; a loop through an operation
// (a counter that is stepped) has values the tracing does not enumerate, so
// the frame it is in fails: the set of values must not be told as complete.
func (t *tracer) cutLoop(enteredAt int, pos ir.Pos, chain []string) {
	if t.opDepth > enteredAt {
		t.stop(KindDepthLimit, "a loop goes through an operation: its values are not worked out", "", pos, chain, nil)
	}
}

// derived traces the operands of an operation on their own and folds the
// operation over the constants they reach. The result is told only when every
// operand reaches constants and the operation can be computed on them; else
// the operation is recorded as not followed (unless an operand already said
// where it stopped). The operands' own values are never told as the result.
func (t *tracer) derived(v *sg.Value, ops []*sg.Value, what string, fn *sg.Func, chain []string, site ir.Pos, depth int, fold func([]provVal) (provVal, bool)) {
	if len(ops) == 0 {
		t.stop(KindDepthLimit, "not followed: "+notFollowed(v), "", v.Pos, chain, nil)
		return
	}
	outer := t.frame
	t.opDepth++
	defer func() { t.opDepth-- }()
	all := make([][]capVal, len(ops))
	complete, said := true, false
	for i, o := range ops {
		f := &frame{}
		t.frame = f
		t.walk(o, fn, chain, site, depth)
		t.frame = outer
		if f.failed {
			said = true
			complete = false
			if outer != nil {
				outer.failed = true
			}
		}
		if len(f.vals) == 0 {
			complete = false
		}
		for _, cv := range f.vals {
			if cv.c == nil {
				complete = false
			}
		}
		all[i] = f.vals
	}
	var out []capVal
	if complete {
		out = foldAll(all, v, fold)
	}
	if out == nil {
		if !said {
			t.stop(KindDepthLimit, "the value is computed by "+what+" and could not be worked out to a constant", "", v.Pos, chain, nil)
		}
		return
	}
	for _, o := range out {
		t.resolved(o.c.text(), o.site, o.cert, o.c)
	}
}

// foldAll applies the operation to every combination of the operands' values
// (a φ gives several); nil when any combination cannot be computed or there
// are too many.
func foldAll(all [][]capVal, v *sg.Value, fold func([]provVal) (provVal, bool)) []capVal {
	combos := 1
	for _, a := range all {
		combos *= len(a)
		if combos > 16 {
			return nil
		}
	}
	var out []capVal
	idx := make([]int, len(all))
	for n := 0; n < combos; n++ {
		args := make([]provVal, len(all))
		cert := CertaintyZeroValue
		var site ir.Pos
		for i, a := range all {
			cv := a[idx[i]]
			args[i] = *cv.c
			if cv.cert == CertaintyLiteral {
				cert = CertaintyLiteral
			}
			if i == 0 {
				site = cv.site
			}
		}
		r, ok := fold(args)
		if !ok {
			return nil
		}
		out = append(out, capVal{&r, cert, site})
		for i := len(idx) - 1; i >= 0; i-- {
			idx[i]++
			if idx[i] < len(all[i]) {
				break
			}
			idx[i] = 0
		}
	}
	return out
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
	if ci, rest, ok := t.enteredBy(fn); ok {
		// The callee is being read for the result of this very call: the
		// parameter is the argument of this call, not of the other callers.
		argv := callArgv(ci)
		if idx >= 0 && idx < len(argv) {
			saved := t.ctx
			t.ctx = rest
			t.walk(argv[idx], ci.Block.Func, chain, ci.Pos, depth+1)
			t.ctx = saved
		}
		return
	}
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

// enteredBy tells whether fn is being read for the result of a call, and
// gives that call and the calls outside it.
func (t *tracer) enteredBy(fn *sg.Func) (*sg.Instr, []provCall, bool) {
	n := len(t.ctx)
	if n == 0 || t.ctx[n-1].callee != fn {
		return nil, nil, false
	}
	return t.ctx[n-1].call, t.ctx[:n-1], true
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
	if vc := call.Callee.ValueCands; call.Callee.Kind == ir.CalleeValue && len(vc) == 1 {
		sym = vc[0].Symbol
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
	case len(callees) == 0 && call.Callee != nil && call.Callee.ValueLimit != "":
		t.stop(KindDepthLimit, call.Callee.ValueLimitDetail(), sym, call.Pos, chain, nil)
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
					saved := t.ctx
					t.ctx = append(t.ctx[:len(t.ctx):len(t.ctx)], provCall{call, b})
					t.walk(ret.Args[idx], b, chain, call.Pos, depth+1)
					t.ctx = saved
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
		// A variable of the function: what is stored into it, unless its
		// address is passed on and others may store into it too.
		if allocEscapes(fn, addr) {
			t.stop(KindDepthLimit, "the address of the variable is passed on: it may be changed elsewhere", "", addr.Pos, chain, nil)
			return
		}
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
	t.nodes++
	if depth > maxProvDepth || t.nodes > maxProvNodes {
		t.stop(KindDepthLimit, "the tracing was cut at its depth or size limit", "", base.Pos, chain, nil)
		return
	}
	k := objKey{base, field}
	if t.objPath == nil {
		t.objPath = map[objKey]int{}
	}
	if d, ok := t.objPath[k]; ok {
		t.cutLoop(d, base.Pos, chain)
		return
	}
	t.objPath[k] = t.opDepth
	defer delete(t.objPath, k)
	chain = append(chain, "."+field)
	switch {
	case base.Kind == sg.KGlobal:
		t.stop(KindGlobalVar, "the field "+field+" of the package variable "+base.Name+"; its direct writers are listed", base.Name+"."+field, base.Pos,
			chain, t.e.writersOf(base))
	case base.Instr != nil && base.Instr.Op == ir.OpAlloc:
		set := false
		// A write that reaches the field in another form: the whole object
		// assigned (from a call, a copy of what a pointer points to, an
		// element), or a part of the field.
		other := false
		for _, blk := range fn.Blocks {
			for _, in := range blk.Instrs {
				switch {
				case in.Op == ir.OpStore && in.Args[0] == base:
					z := in.Args[1]
					switch {
					case z.Kind == sg.KConst && z.Const == nil:
						// The zero value of the type: changes nothing.
					case z.Kind == sg.KResult && z.Instr != nil && z.Instr.Op == ir.OpLoad && z.Instr.Args[0].Instr != nil && z.Instr.Args[0].Instr.Op == ir.OpAlloc:
						// A copy of another variable of the function (a
						// literal is built there and copied): the field is
						// what that variable's field is.
						set = true
						t.walkObject(z.Instr.Args[0], field, read, fn, chain[:len(chain)-1], site, depth+1)
					default:
						other = true
					}
				case in.Op == ir.OpAddrOf && len(in.Args) == 1 && in.Args[0] == base && len(in.IR.Path) > 1 && in.IR.Path[0].Kind == ir.StepField && in.IR.Path[0].Field == field:
					other = true
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
		case other:
			t.stop(KindDepthLimit, "the object that holds the field is assigned as a whole (or a part of the field is set): the field is not followed", "", base.Pos, chain, nil)
		case !set:
			// Nothing sets it and nobody else can: the zero value.
			if read != nil {
				var cp *provVal
				if c, ok := provConst(read); ok {
					cp = &c
				}
				t.resolved(zeroText(read.Class, read.Type), site, CertaintyZeroValue, cp)
			}
		}
	case base.Kind == sg.KParam:
		// The object is handed in: look at what the callers pass.
		idx := base.Param
		if ci, rest, ok := t.enteredBy(fn); ok {
			argv := callArgv(ci)
			if idx >= 0 && idx < len(argv) {
				saved := t.ctx
				t.ctx = rest
				t.walkObject(argv[idx], field, read, ci.Block.Func, chain[:len(chain)-1], ci.Pos, depth+1)
				t.ctx = saved
			}
			return
		}
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
			t.stop(KindDepthLimit, fmt.Sprintf("%s has %d callers; only the first %d are followed", fn.ID, len(sites), maxProvCallers), "", base.Pos, chain, nil)
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
