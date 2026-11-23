package goir

import (
	"go/constant"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
)

func convOperand(in ssa.Instruction) ssa.Value {
	switch in := in.(type) {
	case *ssa.ChangeType:
		return in.X
	case *ssa.Convert:
		return in.X
	case *ssa.ChangeInterface:
		return in.X
	case *ssa.MakeInterface:
		return in.X
	case *ssa.SliceToArrayPointer:
		return in.X
	case *ssa.MultiConvert:
		return in.X
	}
	return nil
}

// unop converts a unary operation. The dereference "*p" is a load; the
// receive "<-ch" is a call.
func (c *conv) unop(in *ssa.UnOp, pos ir.Pos) {
	switch in.Op {
	case token.MUL:
		if info, ok := c.addr[in.X]; ok {
			if info.isIdx {
				c.emit(ir.Instr{Op: ir.OpIndexLoad, Dsts: c.dst(in), Args: []ir.LocalID{info.base, info.index}, Pos: pos, AddrPos: info.pos, Folded: true, CharRead: isByteOrRune(in.Type())})
			} else {
				c.emit(ir.Instr{Op: ir.OpFieldLoad, Dsts: c.dst(in), Args: []ir.LocalID{info.base}, Field: info.field, Pos: pos, AddrPos: info.pos, Folded: true})
			}
			return
		}
		if g, ok := in.X.(*ssa.Global); ok {
			c.noteGlobal(g)
			c.emit(ir.Instr{Op: ir.OpGlobalLoad, Dsts: c.dst(in), Global: globalName(g), Pos: pos})
			return
		}
		c.emit(ir.Instr{Op: ir.OpLoad, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X)}, Pos: pos})
	case token.ARROW:
		dsts := c.dst(in)
		if in.CommaOk {
			dsts = c.tupleDsts(in, 2)
		}
		c.emit(ir.Instr{Op: ir.OpCall, Dsts: dsts, Args: []ir.LocalID{c.val(in.X)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "chan.recv"}, Pos: pos})
	default:
		c.emit(ir.Instr{Op: ir.OpUnop, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X)}, Operator: in.Op.String(), Pos: pos})
	}
}

func (c *conv) store(in *ssa.Store, pos ir.Pos) {
	if info, ok := c.addr[in.Addr]; ok {
		val := c.val(in.Val)
		if info.isIdx {
			c.emit(ir.Instr{Op: ir.OpIndexStore, Args: []ir.LocalID{info.base, info.index, val}, Pos: pos, AddrPos: info.pos, Folded: true})
		} else {
			c.emit(ir.Instr{Op: ir.OpFieldStore, Args: []ir.LocalID{info.base, val}, Field: info.field, Pos: pos, AddrPos: info.pos, Folded: true})
		}
		return
	}
	if g, ok := in.Addr.(*ssa.Global); ok {
		c.noteGlobal(g)
		c.emit(ir.Instr{Op: ir.OpGlobalStore, Args: []ir.LocalID{c.val(in.Val)}, Global: globalName(g), Pos: pos})
		return
	}
	addr := c.val(in.Addr)
	c.emit(ir.Instr{Op: ir.OpStore, Args: []ir.LocalID{addr, c.val(in.Val)}, Pos: pos})
}

// call converts a call, a go statement or a defer statement. v is the value
// of an ordinary call, nil for the others.
func (c *conv) call(ci ssa.CallInstruction, v *ssa.Call, cc *ssa.CallCommon, pos ir.Pos, goStmt, deferStmt bool) {
	in := ir.Instr{Op: ir.OpCall, Go: goStmt, Defer: deferStmt, Pos: pos}
	if v != nil {
		if tup, ok := v.Type().(*types.Tuple); ok {
			if tup.Len() > 0 {
				in.Dsts = c.tupleDsts(v, tup.Len())
			}
		} else {
			in.Dsts = c.dst(v)
		}
	}
	switch {
	case cc.IsInvoke():
		in.Callee = &ir.Callee{Kind: ir.CalleeDynamic, Symbol: cc.Method.FullName(), Candidates: c.candidates(ci)}
		in.Args = append([]ir.LocalID{c.val(cc.Value)}, c.vals(cc.Args)...)
	default:
		switch fn := cc.Value.(type) {
		case *ssa.Builtin:
			// the builtins of package unsafe are named with it ("builtin.unsafe.String"): another extractor's builtin of the
			// same short name is not taken for them (taint.builtin)
			name := fn.Name()
			if obj := fn.Object(); obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "unsafe" {
				name = "unsafe." + name
			}
			in.Callee = &ir.Callee{Kind: ir.CalleeStatic, Symbol: "builtin." + name}
			in.Args = c.vals(cc.Args)
		case *ssa.Function:
			in.Callee = &ir.Callee{Kind: ir.CalleeStatic, Func: ir.FuncID(gossa.FuncID(fn)), Symbol: gossa.Symbol(fn), Recv: fn.Signature.Recv() != nil && len(cc.Args) > 0}
			in.Args = c.vals(cc.Args)
		default:
			in.Callee = &ir.Callee{Kind: ir.CalleeValue}
			if _, closure := cc.Value.(*ssa.MakeClosure); !closure {
				in.Callee.Candidates = c.candidates(ci)
			}
			in.Args = append([]ir.LocalID{c.val(cc.Value)}, c.vals(cc.Args)...)
		}
	}
	c.emit(in)
}

// lowerPhis replaces the φ functions that go/ssa keeps in NaiveForm with
// copies into a local at the end of each predecessor. When a predecessor
// feeds several φ functions, the values go through temporaries first so that
// the copies behave as the parallel assignment a φ block is.
func (c *conv) lowerPhis() {
	for bi, b := range c.fn.Blocks {
		var phis []*ssa.Phi
		for _, in := range b.Instrs {
			if phi, ok := in.(*ssa.Phi); ok {
				phis = append(phis, phi)
			}
		}
		if len(phis) == 0 {
			continue
		}
		_ = bi
		for pi, pred := range b.Preds {
			c.cur = &c.out.Blocks[pred.Index].Instrs
			var temps []ir.LocalID
			for _, phi := range phis {
				arg := c.val(phi.Edges[pi])
				if len(phis) > 1 {
					t := c.newLocal("", phi.Type())
					c.emit(ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{t}, Args: []ir.LocalID{arg}, Pos: c.pos(phi.Pos())})
					arg = t
				}
				temps = append(temps, arg)
			}
			for i, phi := range phis {
				c.emit(ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{c.local(phi)}, Args: []ir.LocalID{temps[i]}, Pos: c.pos(phi.Pos())})
			}
		}
	}
}

func constOf(k *ssa.Const) *ir.Const {
	if k.Value == nil {
		return &ir.Const{Kind: ir.ConstNil}
	}
	switch k.Value.Kind() {
	case constant.String:
		s := constant.StringVal(k.Value)
		if len(s) > MaxConstString {
			return &ir.Const{Kind: ir.ConstString, Value: s[:MaxConstString], Truncated: true}
		}
		return &ir.Const{Kind: ir.ConstString, Value: s}
	case constant.Int:
		return &ir.Const{Kind: ir.ConstInt, Value: k.Value.ExactString()}
	case constant.Float, constant.Complex:
		return &ir.Const{Kind: ir.ConstFloat, Value: k.Value.ExactString()}
	case constant.Bool:
		return &ir.Const{Kind: ir.ConstBool, Value: k.Value.ExactString()}
	}
	return &ir.Const{Kind: ir.ConstNil}
}

func qualify(p *types.Package) string { return p.Path() }

// typeString is the name of a type as the IR writes it. A type defined inside a function whose name is
// also the name of a type alias of the package is told apart by its position (`pkg.Name·N`), so the
// table of aliases (type_aliases) is not applied to it: the two are not the same type (ADR-083).
func typeString(t types.Type) string {
	if !hasShadow(t, 0) {
		return types.TypeString(t, qualify)
	}
	return shadowString(t)
}

// shadowedBy reports whether n is a type defined in a function, under the name of a type alias that its
// package declares.
func shadowedBy(n *types.Named) bool {
	obj := n.Obj()
	if obj == nil || obj.Pkg() == nil || obj.Parent() == nil || obj.Parent() == obj.Pkg().Scope() {
		return false
	}
	o, ok := obj.Pkg().Scope().Lookup(obj.Name()).(*types.TypeName)
	return ok && o.IsAlias()
}

// hasShadow reports whether a type that shadowString writes differently is in t: under the pointer, slice,
// array, map and channel types and the type arguments.
func hasShadow(t types.Type, depth int) bool {
	if depth > 16 {
		return false
	}
	switch t := t.(type) {
	case *types.Named:
		if shadowedBy(t) {
			return true
		}
		for i := 0; i < t.TypeArgs().Len(); i++ {
			if hasShadow(t.TypeArgs().At(i), depth+1) {
				return true
			}
		}
	case *types.Pointer:
		return hasShadow(t.Elem(), depth+1)
	case *types.Slice:
		return hasShadow(t.Elem(), depth+1)
	case *types.Array:
		return hasShadow(t.Elem(), depth+1)
	case *types.Chan:
		return hasShadow(t.Elem(), depth+1)
	case *types.Map:
		return hasShadow(t.Key(), depth+1) || hasShadow(t.Elem(), depth+1)
	}
	return false
}

func shadowString(t types.Type) string {
	switch t := t.(type) {
	case *types.Named:
		base := types.TypeString(t.Origin(), qualify)
		if i := strings.IndexByte(base, '['); i >= 0 {
			base = base[:i]
		}
		if shadowedBy(t) {
			base += "·" + strconv.Itoa(int(t.Obj().Pos()))
		}
		if t.TypeArgs().Len() == 0 {
			return base
		}
		args := make([]string, t.TypeArgs().Len())
		for i := range args {
			args[i] = typeString(t.TypeArgs().At(i))
		}
		return base + "[" + strings.Join(args, ",") + "]"
	case *types.Pointer:
		return "*" + typeString(t.Elem())
	case *types.Slice:
		return "[]" + typeString(t.Elem())
	case *types.Array:
		return "[" + strconv.FormatInt(t.Len(), 10) + "]" + typeString(t.Elem())
	case *types.Chan:
		pre := map[types.ChanDir]string{types.SendRecv: "chan ", types.SendOnly: "chan<- ", types.RecvOnly: "<-chan "}[t.Dir()]
		return pre + typeString(t.Elem())
	case *types.Map:
		return "map[" + typeString(t.Key()) + "]" + typeString(t.Elem())
	}
	return types.TypeString(t, qualify)
}

// classOf is the class of a type (ADR-039).
func classOf(t types.Type) ir.TypeClass {
	if t == nil {
		return ""
	}
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		return ir.ClassPointer
	case *types.Interface:
		return ir.ClassInterface
	case *types.Signature:
		return ir.ClassFunc
	case *types.Slice, *types.Map, *types.Chan:
		return ir.ClassContainer
	case *types.Struct, *types.Array:
		return ir.ClassStruct
	case *types.Basic:
		switch {
		case u.Info()&types.IsBoolean != 0:
			return ir.ClassBool
		case u.Info()&types.IsString != 0:
			return ir.ClassString
		case u.Info()&types.IsNumeric != 0:
			return ir.ClassNumber
		}
	}
	return ir.ClassOther
}

// fieldName names the i-th field of the struct that x (a struct or a pointer
// to one) has.
func fieldName(x types.Type, i int) string {
	t := x
	if pt, ok := t.Underlying().(*types.Pointer); ok {
		t = pt.Elem()
	}
	if st, ok := t.Underlying().(*types.Struct); ok && i < st.NumFields() {
		if n := st.Field(i).Name(); n != "" && n != "_" {
			return n
		}
	}
	return "#" + strconv.Itoa(i)
}

// convKind names the kind of a conversion, as the engine tells them apart.
func convKind(in ssa.Instruction) string {
	switch in.(type) {
	case *ssa.MakeInterface:
		return "make_interface"
	case *ssa.ChangeInterface:
		return "change_interface"
	case *ssa.SliceToArrayPointer:
		return "slice_to_array_pointer"
	}
	return "convert"
}

// displayString is a type as a reader writes it: package names, not paths.
func displayString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string { return p.Name() })
}

// candidates are the functions the call graph says a call may reach.
func (c *conv) candidates(ci ssa.CallInstruction) []ir.FuncID {
	if c.graph == nil {
		return nil
	}
	sites := []ssa.CallInstruction{ci}
	if c.liftedCall != nil {
		if l, ok := c.liftedCall[ci]; ok {
			sites = l
		}
	}
	var out []ir.FuncID
	for _, site := range sites {
		for _, fn := range c.graph.Callees(site) {
			out = append(out, ir.FuncID(gossa.FuncID(fn)))
		}
	}
	if len(sites) > 1 {
		// the callees of each are in order: the join is too
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		w := 0
		for i, id := range out {
			if i == 0 || id != out[w-1] {
				out[w] = id
				w++
			}
		}
		out = out[:w]
	}
	return out
}
