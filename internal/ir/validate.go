package ir

import "fmt"

// shape is how many locals an op writes and reads.
type shape struct {
	dsts, args int // -1: any number
}

var shapes = map[Op]shape{
	OpConst:          {1, 0},
	OpParam:          {1, 0},
	OpCopy:           {1, 1},
	OpUnop:           {1, 1},
	OpBinop:          {1, 2},
	OpConvert:        {1, 1},
	OpTypeAssert:     {-1, 1}, // 1 or 2 dsts
	OpAlloc:          {1, 0},
	OpLoad:           {1, 1},
	OpStore:          {0, 2},
	OpAddrOf:         {1, -1}, // 1 argument, or none for the address of a global
	OpFieldLoad:      {1, 1},
	OpFieldStore:     {0, 2},
	OpIndexLoad:      {-1, 2}, // 1 or 2 dsts: the value, and the found flag of the comma-ok form
	OpIndexStore:     {0, 3},
	OpGlobalLoad:     {1, 0},
	OpGlobalStore:    {0, 1},
	OpMakeClosure:    {1, -1},
	OpCall:           {-1, -1},
	OpExceptionValue: {1, 0},
	OpUnsupported:    {0, 0},
}

// Validate checks the program: every function, and the references between
// them that the stream states (the implementations).
func (p *Program) Validate() error {
	seen := map[FuncID]bool{}
	for _, f := range p.Funcs {
		if f == nil {
			return fmt.Errorf("nil function")
		}
		if seen[f.ID] {
			return fmt.Errorf("function %s is defined twice", f.ID)
		}
		seen[f.ID] = true
		if err := f.Validate(); err != nil {
			return fmt.Errorf("function %s: %w", f.ID, err)
		}
	}
	for _, im := range p.Impls {
		if im.Method == "" {
			return fmt.Errorf("implementations without a method")
		}
	}
	return nil
}

// Validate checks what the core relies on: the blocks form a graph with a
// root, every instruction has the shape of its op, and every local it names
// is declared.
func (f *Func) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("no id")
	}
	if len(f.Blocks) == 0 {
		return fmt.Errorf("no blocks")
	}
	roots := map[BlockID]bool{}
	for _, r := range f.Roots {
		if r < 0 || int(r) >= len(f.Blocks) {
			return fmt.Errorf("root %d is not a block", r)
		}
		roots[r] = true
	}
	if len(f.Roots) > 0 && !roots[0] {
		return fmt.Errorf("the entry block is not a root")
	}
	for _, l := range f.Locals {
		if !l.Class.valid() {
			return fmt.Errorf("local %q has the unknown type class %q", l.Name, l.Class)
		}
	}
	nparams := 0
	for _, pa := range f.Params {
		if !pa.Class.valid() {
			return fmt.Errorf("parameter %q has the unknown type class %q", pa.Name, pa.Class)
		}
		nparams++
	}
	local := func(id LocalID, what string) error {
		if id < 0 || int(id) >= len(f.Locals) {
			return fmt.Errorf("%s: local %d is not declared (%d locals)", what, id, len(f.Locals))
		}
		return nil
	}
	for i, b := range f.Blocks {
		where := fmt.Sprintf("block %d", i)
		if b.ID != BlockID(i) {
			return fmt.Errorf("%s has the id %d", where, b.ID)
		}
		for j, in := range b.Instrs {
			w := fmt.Sprintf("%s, instruction %d (%s)", where, j, in.Op)
			if err := validateInstr(in, nparams, w, local); err != nil {
				return err
			}
		}
		if err := validateTerm(f, b, where, local, roots); err != nil {
			return err
		}
	}
	return nil
}

func validateInstr(in Instr, nparams int, w string, local func(LocalID, string) error) error {
	sh, ok := shapes[in.Op]
	if !ok {
		return fmt.Errorf("%s: unknown op", w)
	}
	for _, d := range in.Dsts {
		if err := local(d, w); err != nil {
			return err
		}
	}
	for _, a := range in.Args {
		if err := local(a, w); err != nil {
			return err
		}
	}
	if sh.dsts >= 0 && len(in.Dsts) != sh.dsts {
		return fmt.Errorf("%s: writes %d locals, want %d", w, len(in.Dsts), sh.dsts)
	}
	if sh.args >= 0 && len(in.Args) != sh.args {
		return fmt.Errorf("%s: reads %d locals, want %d", w, len(in.Args), sh.args)
	}
	if len(in.ArgPos) != 0 && len(in.ArgPos) != len(in.Args) {
		return fmt.Errorf("%s: %d argument positions for %d arguments", w, len(in.ArgPos), len(in.Args))
	}
	need := func(ok bool, what string) error {
		if !ok {
			return fmt.Errorf("%s: %s", w, what)
		}
		return nil
	}
	switch in.Op {
	case OpConst:
		if in.Const == nil {
			return need(false, "no constant")
		}
		switch in.Const.Kind {
		case ConstString, ConstInt, ConstFloat, ConstBool, ConstNil:
		default:
			return need(false, "unknown constant kind "+string(in.Const.Kind))
		}
	case OpParam:
		return need(in.Index >= 0 && in.Index < nparams, fmt.Sprintf("parameter %d of %d", in.Index, nparams))
	case OpUnop, OpBinop:
		return need(in.Operator != "", "no operator")
	case OpTypeAssert:
		if err := need(len(in.Dsts) == 1 || len(in.Dsts) == 2, "writes the value, and the flag of the comma-ok form"); err != nil {
			return err
		}
		return need(in.Type != "", "no type")
	case OpAlloc:
		return need(in.Type != "", "no type")
	case OpAddrOf:
		if in.Global != "" {
			if err := need(len(in.Args) == 0, "the address of a global has no argument"); err != nil {
				return err
			}
		} else if err := need(len(in.Args) == 1, "the address of a local needs a base"); err != nil {
			return err
		}
		for _, s := range in.Path {
			switch s.Kind {
			case StepField:
				if s.Field == "" {
					return need(false, "a field step without a field")
				}
			case StepIndex, StepDeref:
			default:
				return need(false, "unknown path step "+string(s.Kind))
			}
		}
	case OpIndexLoad:
		return need(len(in.Dsts) == 1 || len(in.Dsts) == 2, "writes the value, and the flag of the comma-ok form")
	case OpFieldLoad, OpFieldStore:
		return need(in.Field != "", "no field")
	case OpGlobalLoad, OpGlobalStore:
		return need(in.Global != "", "no global")
	case OpMakeClosure:
		return need(in.Func != "", "no function")
	case OpCall:
		if in.Callee == nil {
			return need(false, "no callee")
		}
		switch in.Callee.Kind {
		case CalleeStatic:
			return need(in.Callee.Symbol != "" || in.Callee.Func != "", "a static callee with neither a function nor a symbol")
		case CalleeDynamic:
			if err := need(in.Callee.Symbol != "", "a dynamic callee without a method"); err != nil {
				return err
			}
			return need(len(in.Args) >= 1, "a dynamic call without a receiver")
		case CalleeValue:
			return need(len(in.Args) >= 1, "a call of a value without the value")
		default:
			return need(false, "unknown callee kind "+string(in.Callee.Kind))
		}
	case OpUnsupported:
		return need(in.Reason != "", "no reason")
	}
	return nil
}

func validateTerm(f *Func, b Block, where string, local func(LocalID, string) error, roots map[BlockID]bool) error {
	for _, e := range b.Succs {
		if e.To < 0 || int(e.To) >= len(f.Blocks) {
			return fmt.Errorf("%s: an edge to %d, which is not a block", where, e.To)
		}
		if roots[e.To] || (len(roots) == 0 && e.To == 0) {
			return fmt.Errorf("%s: an edge to the root block %d", where, e.To)
		}
		switch e.Kind {
		case EdgeNormal, EdgeTrue, EdgeFalse, EdgeException, EdgeDefer, EdgeBack:
		default:
			return fmt.Errorf("%s: unknown edge kind %q", where, e.Kind)
		}
	}
	count := func(k ...EdgeKind) int {
		n := 0
		for _, e := range b.Succs {
			for _, kk := range k {
				if e.Kind == kk {
					n++
				}
			}
		}
		return n
	}
	t := b.Term
	switch t.Kind {
	case TermJump:
		if n := count(EdgeNormal, EdgeBack); n != 1 {
			return fmt.Errorf("%s: a jump has %d normal edges, want 1", where, n)
		}
	case TermBranch:
		if err := local(t.Cond, where+", branch condition"); err != nil {
			return err
		}
		if count(EdgeTrue) != 1 || count(EdgeFalse) != 1 || count(EdgeNormal, EdgeBack) != 0 {
			return fmt.Errorf("%s: a branch needs exactly one true and one false edge", where)
		}
	case TermReturn, TermThrow, TermUnreachable:
		if count(EdgeNormal, EdgeBack, EdgeTrue, EdgeFalse) != 0 {
			return fmt.Errorf("%s: a %s has no normal edge", where, t.Kind)
		}
		for _, v := range t.Values {
			if err := local(v, where+", "+string(t.Kind)); err != nil {
				return err
			}
		}
		if t.Kind == TermThrow && len(t.Values) != 1 {
			return fmt.Errorf("%s: a throw carries one value", where)
		}
	default:
		return fmt.Errorf("%s: unknown terminator %q", where, t.Kind)
	}
	if count(EdgeException) > 0 {
		last := len(b.Instrs) - 1
		if t.Kind != TermThrow && (last < 0 || b.Instrs[last].Op != OpCall) {
			return fmt.Errorf("%s: an exception edge from a block that ends neither in a call nor in a throw", where)
		}
	}
	return nil
}
