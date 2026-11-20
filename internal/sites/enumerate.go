package sites

import (
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irssa"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/valuecalls"
)

// IRRef is the place of a site in the IR.
type IRRef struct {
	Func  *ir.Func
	Block int
	Instr int
	// Param is the index of the parameter of a parameter source, else -1.
	Param int
}

// IRInstr identifies an instruction of the IR.
type IRInstr struct {
	Func         ir.FuncID
	Block, Instr int
}

// IRParam identifies a parameter of a function of the IR.
type IRParam struct {
	Func  ir.FuncID
	Index int
}

// Enumerate finds all source and sink sites in the functions of the program
// that belong to the code being analysed. The texts come from the language's
// Texts. Level L0 (rule matching on resolved symbols) plus the L1 literal
// mark: it needs neither a call graph nor any data flow analysis, so it is
// fast. Every site has a stable ID and a fingerprint (ADR-028).
func Enumerate(prog *ir.Program, texts ir.Texts, rules *model.Rules) *Result {
	valuecalls.Resolve(prog)
	res := &Result{
		ByIRInstr: map[IRInstr][]*Site{},
		ByIRParam: map[IRParam][]*Site{},
		ByID:      map[string]*Site{},
	}
	for _, fn := range prog.Funcs {
		if fn.Dep {
			continue
		}
		var raw []*Site
		order := 0
		add := func(s *Site) {
			s.Func = string(fn.ID)
			s.order = order
			order++
			raw = append(raw, s)
		}
		declared := 0
		for _, p := range fn.Params {
			if !p.Captured {
				declared++
			}
		}
		paramPos := func(i int) ir.Pos {
			for _, in := range fn.Blocks[0].Instrs {
				if in.Op == ir.OpParam && in.Index == i {
					return in.Pos
				}
			}
			return ir.Pos{}
		}

		if sym := fn.Symbol; sym != "" {
			for _, src := range rules.ParamFuncs[sym] {
				off := 0
				if fn.Recv {
					off = 1
					if src.Recv && declared > 0 {
						add(&Site{Role: RoleSource, Kind: model.KindParam, Symbol: sym, Labels: src.Labels, Src: []model.Source{src}, irpos: paramPos(0), IR: &IRRef{Func: fn, Param: 0}})
					}
				}
				for _, i := range src.Params {
					if i >= 0 && i+off < declared {
						add(&Site{Role: RoleSource, Kind: model.KindParam, Symbol: sym, Labels: src.Labels, Src: []model.Source{src}, irpos: paramPos(i + off), IR: &IRRef{Func: fn, Param: i + off}})
					}
				}
			}
		}
		for i, prm := range fn.Params {
			if prm.Captured {
				continue
			}
			t := string(prm.Type)
			for _, src := range rules.ParamTypes {
				if src.Type == t {
					add(&Site{Role: RoleSource, Kind: model.KindParamType, Symbol: t, Labels: src.Labels, Src: []model.Source{src}, irpos: paramPos(i), IR: &IRRef{Func: fn, Param: i}})
				}
			}
		}
		for bi, b := range fn.Blocks {
			for ii, in := range b.Instrs {
				at := &IRRef{Func: fn, Block: bi, Instr: ii, Param: -1}
				switch in.Op {
				case ir.OpCall:
					if in.Callee.Kind == ir.CalleeValue {
						// A call of a function value: the rules apply to the functions
						// it may reach, each its own site (ADR-045).
						vin := in
						vin.Args = in.Args[min(1, len(in.Args)):]
						for _, c := range in.Callee.ValueCands {
							sym := c.Symbol
							if sym == "" {
								continue
							}
							if srcs := rules.CallSources[sym]; len(srcs) > 0 {
								add(&Site{Role: RoleSource, Kind: model.KindCallResult, Symbol: sym, Labels: labelsOf(srcs), IR: at, Src: srcs, irpos: in.Pos, Callee: string(c.ID)})
							}
							if sinks := sinksAt(fn, vin, c.Recv, rules.Sinks[sym]); len(sinks) > 0 {
								add(&Site{Role: RoleSink, Kind: "call", Symbol: sym, Category: sinks[0].Category, Labels: acceptsOf(sinks), IR: at, Sink: sinks, irpos: in.Pos, Callee: string(c.ID)})
							}
						}
						continue
					}
					sym := in.Callee.Symbol
					if sym == "" {
						continue
					}
					if srcs := rules.CallSources[sym]; len(srcs) > 0 {
						add(&Site{Role: RoleSource, Kind: model.KindCallResult, Symbol: sym, Labels: labelsOf(srcs), IR: at, Src: srcs, irpos: in.Pos})
					}
					if sinks := sinksAt(fn, in, in.Callee.Kind == ir.CalleeDynamic || (in.Callee.Kind == ir.CalleeStatic && in.Callee.Recv), rules.Sinks[sym]); len(sinks) > 0 {
						add(&Site{Role: RoleSink, Kind: "call", Symbol: sym, Category: sinks[0].Category, Labels: acceptsOf(sinks), IR: at, Sink: sinks, irpos: in.Pos})
					}
				case ir.OpFieldLoad, ir.OpFieldStore, ir.OpAddrOf:
					field := in.Field
					if in.Op == ir.OpAddrOf {
						if len(in.Path) == 0 || in.Path[0].Kind != ir.StepField {
							continue
						}
						field = in.Path[0].Field
					}
					if len(in.Args) == 0 {
						continue
					}
					key := fieldKeyOf(string(fn.Locals[in.Args[0]].Type), field)
					if key == "" {
						continue
					}
					srcs := rules.Fields[key]
					if i := strings.LastIndexByte(key, '.'); i > 0 {
						// "pkg.Type.*": any field of the type
						srcs = append(append([]model.Source(nil), srcs...), rules.Fields[key[:i]+".*"]...)
					}
					if len(srcs) > 0 {
						pos := in.Pos
						if !in.AddrPos.IsZero() {
							pos = in.AddrPos
						}
						add(&Site{Role: RoleSource, Kind: model.KindField, Symbol: key, Labels: labelsOf(srcs), IR: at, Src: srcs, irpos: pos})
					}
				}
			}
		}
		if len(raw) == 0 {
			continue
		}
		res.finishIR(fn, texts, raw)
	}
	sort.SliceStable(res.Sites, func(i, j int) bool {
		a, b := res.Sites[i], res.Sites[j]
		if a.Position.File != b.Position.File {
			return a.Position.File < b.Position.File
		}
		if a.Position.Offset != b.Position.Offset {
			return a.Position.Offset < b.Position.Offset
		}
		return a.ID < b.ID
	})
	return res
}

// fieldKeyOf is the key of a field in the rules, "pkg/path.Type.field", from
// the type of the base (a struct or a pointer to one) as the IR writes it.
func fieldKeyOf(typ, field string) string {
	typ = strings.TrimPrefix(typ, "*")
	if typ == "" || field == "" {
		return ""
	}
	if i := strings.IndexByte(typ, '['); i >= 0 {
		typ = typ[:i] // an instantiated type: the name of the generic type
	}
	// A named type has the form "path.Name"; the others (struct{...}, []T,
	// map[...]) are not.
	dot := strings.LastIndexByte(typ, '.')
	if dot <= 0 || strings.ContainsAny(typ, " {}()*") {
		return ""
	}
	return typ + "." + field
}

func posLess(a, b ir.Pos) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	return a.Offset < b.Offset
}

func (res *Result) finishIR(fn *ir.Func, texts ir.Texts, raw []*Site) {
	sort.SliceStable(raw, func(i, j int) bool {
		if raw[i].irpos != raw[j].irpos {
			return posLess(raw[i].irpos, raw[j].irpos)
		}
		return raw[i].order < raw[j].order
	})
	body := fingerprint.Hash(texts.Body(fn.ID))
	ordinals := map[string]int{}
	identical := map[string]int{}
	var form *irssa.Form
	for _, s := range raw {
		s.Position = s.irpos.Position()
		s.Snippet = texts.Snippet(s.irpos)
		expr := texts.Expr(s.irpos)
		exprHash := fingerprint.Hash(expr)

		okey := s.Role + "|" + s.Symbol
		ordinals[okey]++
		ord := ordinals[okey]
		ikey := okey + "|" + exprHash
		identical[ikey]++
		iord := identical[ikey]

		structural := fingerprint.Hash(s.Func, s.Role, s.Symbol, itoa(ord))
		s.Fingerprint = map[string]string{
			fingerprint.Structural: structural,
			fingerprint.Content:    fingerprint.Hash(s.Func, s.Role, s.Symbol, exprHash, itoa(iord)),
			fingerprint.Expr:       exprHash,
			fingerprint.Body:       body,
		}
		s.ID = fingerprint.IDPrefix + structural

		if s.Role == RoleSink && s.IR != nil && s.IR.Param < 0 {
			if form == nil {
				form, _ = irssa.Build(fn)
			}
			lit := form != nil && literalOnlyIR(form, s)
			s.LiteralOnly = &lit
		}
		res.Sites = append(res.Sites, s)
		res.ByID[s.ID] = s
		if s.IR.Param >= 0 {
			k := IRParam{fn.ID, s.IR.Param}
			res.ByIRParam[k] = append(res.ByIRParam[k], s)
		} else {
			k := IRInstr{fn.ID, s.IR.Block, s.IR.Instr}
			res.ByIRInstr[k] = append(res.ByIRInstr[k], s)
		}
	}
}

// literalOnlyIR is the L1 mark over the IR: every dangerous argument of the
// sink is a compile-time constant.
func literalOnlyIR(form *irssa.Form, s *Site) bool {
	at := s.IR
	in := at.Func.Blocks[at.Block].Instrs[at.Instr]
	if in.Op != ir.OpCall {
		return false
	}
	hasRecv, argStart, recvAt := false, 0, 0
	switch {
	case in.Callee.Kind == ir.CalleeDynamic || (in.Callee.Kind == ir.CalleeStatic && in.Callee.Recv):
		hasRecv, argStart = true, 1
	case in.Callee.Kind == ir.CalleeValue:
		argStart = 1
		for _, c := range in.Callee.ValueCands {
			if string(c.ID) == s.Callee && c.Recv && len(in.Args) > 1 {
				hasRecv, argStart, recvAt = true, 2, 1
			}
		}
	}
	nargs := len(in.Args) - argStart
	var checked int
	for _, i := range SinkArgs(s.Sink, nargs) {
		checked++
		if !isConstantRef(form, form.Operand(at.Block, at.Instr, argStart+i), 0) {
			return false
		}
	}
	for _, sk := range s.Sink {
		if sk.Recv {
			checked++
			if !hasRecv || !isConstantRef(form, form.Operand(at.Block, at.Instr, recvAt), 0) {
				return false
			}
		}
	}
	return checked > 0
}

// isConstantRef reports whether a value is a compile-time constant, possibly
// converted, or a slice literal made only of constants (as the variadic
// arguments are passed). It is gossa.IsConstant over the IR.
func isConstantRef(form *irssa.Form, r irssa.Ref, depth int) bool {
	if depth > 8 {
		return false
	}
	switch r.Kind {
	case irssa.RefZero:
		// The zero value of a variable: a constant (go/ssa makes it one).
		return true
	case irssa.RefInstr:
	default:
		return false
	}
	f := form.Func
	in := f.Blocks[r.Block].Instrs[r.Index]
	switch in.Op {
	case ir.OpConst:
		return true
	case ir.OpConvert:
		return isConstantRef(form, form.Operand(r.Block, r.Index, 0), depth+1)
	case ir.OpBinop:
		return isConstantRef(form, form.Operand(r.Block, r.Index, 0), depth+1) &&
			isConstantRef(form, form.Operand(r.Block, r.Index, 1), depth+1)
	case ir.OpCopy:
		if !in.View {
			return false
		}
		base := form.Operand(r.Block, r.Index, 0)
		if base.Kind != irssa.RefInstr || f.Blocks[base.Block].Instrs[base.Index].Op != ir.OpAlloc {
			return false
		}
		alloc := f.Blocks[base.Block].Instrs[base.Index].Dsts[0]
		for bi, b := range f.Blocks {
			for ii, u := range b.Instrs {
				for k, a := range u.Args {
					if a != alloc {
						continue
					}
					switch {
					case u.Op == ir.OpCopy && u.View:
					case u.Op == ir.OpIndexStore && k == 0:
						if !isConstantRef(form, form.Operand(bi, ii, 2), depth+1) {
							return false
						}
					default:
						return false
					}
				}
			}
			for _, a := range b.Term.Values {
				if a == alloc {
					return false
				}
			}
		}
		return true
	}
	return false
}

// sinksAt keeps the sinks whose condition on the type of an argument holds at
// this call. hasRecv says that the first of in.Args is the receiver: when_arg_type counts the arguments
// without it, as args does, and recv: true is the condition on it.
func sinksAt(fn *ir.Func, in ir.Instr, hasRecv bool, sinks []model.Sink) []model.Sink {
	var out []model.Sink
	for _, s := range sinks {
		if c := s.WhenArgType; c != nil {
			at := c.Arg
			switch {
			case c.Recv:
				at = 0
				if !hasRecv {
					continue
				}
			case hasRecv:
				at++
			}
			if at < 0 || at >= len(in.Args) || !argTypeHolds(fn, in.Args[at], c) {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

// argTypeHolds checks the condition against the type of the local and, when the
// local is an interface made by a conversion (an http.ResponseWriter passed as
// an io.Writer), against the type that was converted.
func argTypeHolds(fn *ir.Func, l ir.LocalID, c *model.ArgTypeCond) bool {
	if c.Holds(string(fn.Locals[l].Type)) {
		return true
	}
	for depth := 0; depth < 4; depth++ {
		src := ir.LocalID(-1)
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpConvert || len(in.Args) != 1 {
					continue
				}
				for _, d := range in.Dsts {
					if d == l {
						src = in.Args[0]
					}
				}
			}
		}
		if src < 0 {
			return false
		}
		if c.Holds(string(fn.Locals[src].Type)) {
			return true
		}
		l = src
	}
	return false
}
