// Package sg is the SSA graph the analyses run on: the neutral IR of every
// function, with its variables turned into values by internal/irssa. It is
// where the language neutral core meets the engine: a node is a value or an
// instruction, with pointers for identity, operands resolved to the values
// that define them, and blocks with predecessors and dominators.
//
// What does not appear in it: constants and parameters are values but not
// instructions; the loads and stores of promoted variables are gone (their
// values are the values stored); the copies that lower a source language's φ
// are gone (the φ takes the copied values).
package sg

import (
	"github.com/yagihash/rivus/internal/ir"
)

// Op is what an instruction does. It is an IR op, or one of the following.
type Op = ir.Op

const (
	OpPhi         Op = "phi"
	OpJump        Op = "jump"
	OpBranch      Op = "branch"
	OpReturn      Op = "return"
	OpThrow       Op = "throw"
	OpUnreachable Op = "unreachable"
)

// ValueKind says what a Value is.
type ValueKind uint8

const (
	// KResult is a result of an instruction (Instr, Index).
	KResult ValueKind = iota
	// KPhi is the result of a φ instruction.
	KPhi
	// KParam is a declared parameter (the receiver first).
	KParam
	// KFreeVar is a value a closure captured.
	KFreeVar
	// KConst is a constant (Const), or the zero value of a variable.
	KConst
	// KFunc is a function used as a value (Fn, FuncID).
	KFunc
	// KGlobal is the address of a package variable (Name).
	KGlobal
	// KUndef is nothing: no definition reaches.
	KUndef
)

// Value is a thing that has a value: an operand names one.
type Value struct {
	Kind  ValueKind
	Name  string
	Type  ir.TypeID
	Class ir.TypeClass
	// Instr and Index: the instruction that wrote a result, and which of its
	// results (KResult), or the φ instruction (KPhi).
	Instr *Instr
	Index int
	// Func is the function the value lives in (nil for a global or a
	// function value).
	Func *Func
	// Const is the literal of a KConst (nil for a zero value).
	Const *ir.Const
	// FuncID and Fn are the function of a KFunc; Fn is nil if the function
	// has no body in the program.
	FuncID ir.FuncID
	Fn     *Func
	// Param is the position of a KParam or a KFreeVar (free variables count
	// after the parameters).
	Param int
	// Pos is where the value is made, when known.
	Pos ir.Pos
}

// IsPointerLike reports whether the value refers to storage: its location
// is what it points to, not the value itself.
func (v *Value) IsPointerLike() bool { return PointerLike(v.Class) }

// PointerLike is the class test of the engine.
func PointerLike(c ir.TypeClass) bool {
	return c == ir.ClassPointer || c == ir.ClassContainer
}

// Instr is an instruction or a terminator.
type Instr struct {
	Op    Op
	Block *Block
	// Index is the position in Block.Instrs.
	Index int
	Dsts  []*Value
	Args  []*Value
	// IR is what the analysis reads of the IR instruction (nil for a φ and a
	// terminator). It is a copy of the few fields: the instruction itself is
	// not kept, so that the IR of the program can be dropped.
	IR *InstrInfo
	// IRBlock and IRIndex are where it is in the IR (-1 for a φ).
	IRBlock, IRIndex int
	Pos              ir.Pos
	// AddrPos is the position of the folded address computation.
	AddrPos ir.Pos

	// For a call:
	Callee *ir.Callee
	// Static is the function a static call, or a call of a closure made
	// right there, reaches; StaticID is its ID even when it has no body.
	Static   *Func
	StaticID ir.FuncID
	// Bindings are the values the closure made right there captured.
	Bindings []*Value
	// Candidates are the functions a dynamic call may reach (the IDs, and the
	// functions that have a body).
	Candidates   []ir.FuncID
	CandidateFns []*Func
	// CandidateBindings are the values a candidate closure captured, when the
	// candidate was found from the closure being made (a call of a function
	// value).
	CandidateBindings map[*Func][]*Value
}

// Recv is the receiver of a call, or nil.
func (in *Instr) Recv() *Value {
	if in.Callee == nil {
		return nil
	}
	switch {
	case in.Callee.Kind == ir.CalleeDynamic, in.Callee.Kind == ir.CalleeStatic && in.Callee.Recv:
		if len(in.Args) > 0 {
			return in.Args[0]
		}
	}
	return nil
}

// CallArgs are the arguments of a call, the receiver excluded.
func (in *Instr) CallArgs() []*Value {
	if in.Callee == nil {
		return nil
	}
	switch {
	case in.Callee.Kind == ir.CalleeDynamic, in.Callee.Kind == ir.CalleeValue, in.Callee.Kind == ir.CalleeStatic && in.Callee.Recv:
		if len(in.Args) > 0 {
			return in.Args[1:]
		}
		return nil
	}
	return in.Args
}

// Block is a basic block.
type Block struct {
	Func  *Func
	Index int
	// Instrs starts with the φ instructions and ends with the terminator.
	Instrs []*Instr
	Preds  []*Block
	Succs  []*Block
	// Idom is the immediate dominator (nil for a root).
	Idom *Block
	// Edges are the edges out of the block with their kinds (in step with
	// Succs).
	Edges []ir.Edge

	dom map[*Block]bool
}

// Dominates reports whether every path from a root to o passes through b
// (a block dominates itself).
func (b *Block) Dominates(o *Block) bool {
	for x := o; x != nil; x = x.Idom {
		if x == b {
			return true
		}
	}
	return false
}

// Terminator is the last instruction.
func (b *Block) Terminator() *Instr {
	if len(b.Instrs) == 0 {
		return nil
	}
	return b.Instrs[len(b.Instrs)-1]
}

// InstrInfo is the part of an IR instruction that the analysis reads.
type InstrInfo struct {
	Operator string
	Field    string
	Global   string
	Path     []ir.Step
	Func     ir.FuncID
	View     bool
	Folded   bool
	Keyed    bool
	Go       bool
}

// Func is a function of the program.
type Func struct {
	ID string
	// IR is the function without its blocks and locals (they are in Blocks).
	IR     *ir.Func
	Prog   *Program
	Symbol string
	// Recv is true when Params[0] is the receiver.
	Recv     bool
	Dep      bool
	Exported bool
	Entry    bool
	Pos      ir.Pos
	Params   []*Value
	FreeVars []*Value
	Blocks   []*Block
}

// Program is the set of functions.
type Program struct {
	IR      *ir.Program
	Funcs   []*Func
	ByID    map[string]*Func
	Globals map[string]*Value
	// GlobalPos is where each package variable is declared.
	GlobalPos map[string]ir.Pos
}

// Global is the value that stands for a package variable.
func (p *Program) Global(name string) *Value {
	if g, ok := p.Globals[name]; ok {
		return g
	}
	g := &Value{Kind: KGlobal, Name: name, Pos: p.GlobalPos[name]}
	p.Globals[name] = g
	return g
}

// Less is a fixed order of values, for places that must not depend on the
// order of a map: by the function, the kind, the name, and where the value is
// defined.
func (v *Value) Less(o *Value) bool {
	fid := func(x *Value) string {
		if x.Func != nil {
			return x.Func.ID
		}
		return ""
	}
	if a, b := fid(v), fid(o); a != b {
		return a < b
	}
	if v.Kind != o.Kind {
		return v.Kind < o.Kind
	}
	if v.Name != o.Name {
		return v.Name < o.Name
	}
	if v.Param != o.Param {
		return v.Param < o.Param
	}
	site := func(x *Value) (int, int, int) {
		if x.Instr != nil {
			return x.Instr.IRBlock, x.Instr.IRIndex, x.Index
		}
		return -1, -1, x.Index
	}
	ab, ai, ad := site(v)
	bb, bi, bd := site(o)
	if ab != bb {
		return ab < bb
	}
	if ai != bi {
		return ai < bi
	}
	return ad < bd
}

// Reachable is the set of functions the program can reach from where it
// starts: the entry points (main, the initialisation of the packages), or, for
// a program that has none (a library), the exported functions of the code that
// was read. The edges are the calls, the candidates of the calls through
// interfaces and function values, and the functions named as values. It is nil
// when nothing says where the program starts.
func (p *Program) Reachable() map[*Func]bool {
	var work []*Func
	reach := map[*Func]bool{}
	add := func(f *Func) {
		if f != nil && !reach[f] {
			reach[f] = true
			work = append(work, f)
		}
	}
	for _, f := range p.Funcs {
		if f.Entry {
			add(f)
		}
	}
	if len(work) == 0 {
		for _, f := range p.Funcs {
			if f.Exported && !f.Dep {
				add(f)
			}
		}
	}
	if len(work) == 0 {
		return nil
	}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				add(in.Static)
				for _, c := range in.CandidateFns {
					add(c)
				}
				for _, a := range in.Args {
					if a != nil && a.Kind == KFunc {
						add(a.Fn)
					}
				}
			}
		}
	}
	return reach
}
