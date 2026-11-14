package goir

import (
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

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
				c.emit(ir.Instr{Op: ir.OpIndexLoad, Dsts: c.dst(in), Args: []ir.LocalID{info.base, info.index}, Pos: pos, AddrPos: info.pos})
			} else {
				c.emit(ir.Instr{Op: ir.OpFieldLoad, Dsts: c.dst(in), Args: []ir.LocalID{info.base}, Field: info.field, Pos: pos, AddrPos: info.pos})
			}
			return
		}
		if g, ok := in.X.(*ssa.Global); ok {
			c.emit(ir.Instr{Op: ir.OpGlobalLoad, Dsts: c.dst(in), Global: globalName(g), Pos: pos})
			return
		}
		c.emit(ir.Instr{Op: ir.OpLoad, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X)}, Pos: pos})
	case token.ARROW:
		dsts := c.dst(in)
		if in.CommaOk {
			dsts = c.tupleDsts(in, 2)
		}
		c.emit(ir.Instr{Op: ir.OpCall, Dsts: dsts, Args: []ir.LocalID{c.val(in.X)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "go/chan.recv"}, Pos: pos})
	default:
		c.emit(ir.Instr{Op: ir.OpUnop, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X)}, Operator: in.Op.String(), Pos: pos})
	}
}

func (c *conv) store(in *ssa.Store, pos ir.Pos) {
	if info, ok := c.addr[in.Addr]; ok {
		val := c.val(in.Val)
		if info.isIdx {
			c.emit(ir.Instr{Op: ir.OpIndexStore, Args: []ir.LocalID{info.base, info.index, val}, Pos: pos, AddrPos: info.pos})
		} else {
			c.emit(ir.Instr{Op: ir.OpFieldStore, Args: []ir.LocalID{info.base, val}, Field: info.field, Pos: pos, AddrPos: info.pos})
		}
		return
	}
	if g, ok := in.Addr.(*ssa.Global); ok {
		c.emit(ir.Instr{Op: ir.OpGlobalStore, Args: []ir.LocalID{c.val(in.Val)}, Global: globalName(g), Pos: pos})
		return
	}
	addr := c.val(in.Addr)
	c.emit(ir.Instr{Op: ir.OpStore, Args: []ir.LocalID{addr, c.val(in.Val)}, Pos: pos})
}

// call converts a call, a go statement or a defer statement. v is the value
// of an ordinary call, nil for the others.
func (c *conv) call(v *ssa.Call, cc *ssa.CallCommon, pos ir.Pos, goStmt, deferStmt bool) {
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
		in.Callee = &ir.Callee{Kind: ir.CalleeDynamic, Symbol: cc.Method.FullName()}
		in.Args = append([]ir.LocalID{c.val(cc.Value)}, c.vals(cc.Args)...)
	default:
		switch fn := cc.Value.(type) {
		case *ssa.Builtin:
			in.Callee = &ir.Callee{Kind: ir.CalleeStatic, Symbol: "builtin." + fn.Name()}
			in.Args = c.vals(cc.Args)
		case *ssa.Function:
			in.Callee = &ir.Callee{Kind: ir.CalleeStatic, Func: ir.FuncID(gossa.FuncID(fn)), Symbol: gossa.Symbol(fn), Recv: fn.Signature.Recv() != nil && len(cc.Args) > 0}
			in.Args = c.vals(cc.Args)
		default:
			in.Callee = &ir.Callee{Kind: ir.CalleeValue}
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

func typeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string { return p.Path() })
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
	case *types.Slice, *types.Array, *types.Map, *types.Chan:
		return ir.ClassContainer
	case *types.Struct:
		return ir.ClassStruct
	case *types.Basic:
		if u.Kind() == types.UnsafePointer {
			return ir.ClassPointer
		}
		return ir.ClassScalar
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
