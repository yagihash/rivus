package irtaint

import (
	"go/constant"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// maxGuards bounds the guards of one flow (ADR-006). Beyond it the flow is
// kept without guards, which says "under any condition": more than the
// truth, never less.
const maxGuards = 4

// Guard is a branch condition that decides whether a flow happens, and that
// depends on a parameter of the function: the caller controls it. rivus does
// not evaluate it. It writes down which flow goes with which outcome.
type Guard struct {
	// Func is the function the condition is in.
	Func string
	// Params are the parameters the condition depends on.
	Params []int
	// Cond is the condition as written in the source.
	Cond string
	// Polarity is the branch taken: true for the "then" side.
	Polarity bool
	// Call is set for a guard seen from a caller: the call that gives the
	// parameters their values.
	Call *GuardCall

	fn    *sg.Func
	pos   ir.Pos    // where the condition is
	block int       // the block holding the branch
	cond  *sg.Value // the condition itself, to evaluate it for constants
	k     string    // the key, computed once
}

// GuardCall is the call site that instantiates the parameters of a guard.
type GuardCall struct {
	Caller  string
	Snippet string // the call as written
	Args    []GuardArg

	fn    *sg.Func
	instr *sg.Instr
	pos   ir.Pos
}

// GuardArg is the argument passed for a parameter the guard depends on.
type GuardArg struct {
	Param int
	// ArgIndex is the position of the argument in the call as written
	// (the receiver is not counted; -1 for the receiver).
	ArgIndex int

	value *sg.Value
}

// key identifies the condition (without the branch taken).
func (g Guard) condKey() string {
	return g.Func + "|" + strconv.Itoa(g.block) + "|" + posKey(g.pos)
}

func (g Guard) key() string {
	if g.k != "" {
		return g.k
	}
	return g.computeKey()
}

func (g Guard) computeKey() string {
	k := g.condKey() + "|" + strconv.FormatBool(g.Polarity)
	if g.Call != nil {
		k += "|" + posKey(g.Call.pos)
	}
	return k
}

func guardsKey(gs []Guard) string {
	keys := make([]string, len(gs))
	for i, g := range gs {
		keys[i] = g.key()
	}
	sort.Strings(keys)
	return strings.Join(keys, ";")
}

func sameGuards(a, b []Guard) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 || &a[0] == &b[0] {
		return true
	}
	for _, x := range a {
		found := false
		for _, y := range b {
			if x.key() == y.key() {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// unionGuards merges guard lists without duplicates, in a stable order.
func unionGuards(lists ...[]Guard) []Guard {
	// Most flows have no guard at all, and most of the rest have one list.
	nonEmpty := 0
	var only []Guard
	for _, l := range lists {
		if len(l) > 0 {
			nonEmpty++
			only = l
		}
	}
	switch nonEmpty {
	case 0:
		return nil
	case 1:
		if len(only) == 1 {
			return only
		}
	}
	var out []Guard
	seen := map[string]bool{}
	for _, l := range lists {
		for _, g := range l {
			if !seen[g.key()] {
				seen[g.key()] = true
				out = append(out, g)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}

// condParams returns the parameters (and free variables) the value is
// computed from, within the function. The recursion stays inside the
// function: it follows operands only.
func (s *state) condParams(v *sg.Value) []int {
	set := map[int]bool{}
	seen := map[*sg.Value]bool{}
	var walk func(v *sg.Value, depth int)
	walk = func(v *sg.Value, depth int) {
		if v == nil || seen[v] || depth > 10 {
			return
		}
		seen[v] = true
		switch v.Kind {
		case sg.KParam, sg.KFreeVar:
			if i, ok := s.paramRoot[own(v)]; ok {
				set[i] = true
			}
			return
		case sg.KConst, sg.KGlobal, sg.KFunc, sg.KUndef:
			return
		}
		if v.Instr == nil {
			return
		}
		for _, op := range v.Instr.Args {
			walk(op, depth+1)
		}
	}
	walk(v, 0)
	out := make([]int, 0, len(set))
	for i := range set {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// branchGuard builds the guard of the branch that ends block d, for the
// side taken (true: the "then" successor). The result is empty when the
// condition does not depend on a parameter.
func (s *state) branchGuard(d *sg.Block, side bool) (Guard, bool) {
	if len(d.Instrs) == 0 {
		return Guard{}, false
	}
	ifi := d.Instrs[len(d.Instrs)-1]
	if ifi.Op != sg.OpBranch || len(d.Succs) != 2 {
		return Guard{}, false
	}
	if s.condCache == nil {
		s.condCache = map[*sg.Block]*Guard{}
	}
	base, cached := s.condCache[d]
	if !cached {
		params := s.condParams(ifi.Args[0])
		if len(params) == 0 {
			s.condCache[d] = nil
		} else {
			g := &Guard{Func: s.eng.funcID(s.fn), Params: params, fn: s.fn, block: d.Index, cond: ifi.Args[0]}
			g.pos, g.Cond = s.condSource(d)
			s.condCache[d] = g
		}
		base = s.condCache[d]
	}
	if base == nil {
		return Guard{}, false
	}
	g := *base
	g.Polarity = side
	g.k = g.computeKey()
	return g, true
}

// condSource finds the source text of the condition of the branch ending d,
// from a position inside one of its successors.
func (s *state) condSource(d *sg.Block) (ir.Pos, string) {
	for _, succ := range d.Succs {
		for _, in := range succ.Instrs {
			if p := in.Pos; !p.IsZero() {
				if txt := s.eng.texts.Cond(p); txt != "" {
					return p, txt
				}
			}
		}
	}
	return ir.Pos{}, ""
}

// blockGuards are the parameter dependent conditions every execution of the
// block has passed: those of the branches whose side leads only to it.
func (s *state) blockGuards(b *sg.Block) []Guard {
	if len(s.guardBranches()) == 0 {
		return nil
	}
	if s.blockGuardCache == nil {
		s.blockGuardCache = map[*sg.Block][]Guard{}
	}
	if gs, ok := s.blockGuardCache[b]; ok {
		return gs
	}
	var out []Guard
	for _, d := range s.guardBranches() {
		if len(d.Succs) != 2 || d.Succs[0] == d.Succs[1] {
			continue
		}
		for side, succ := range d.Succs {
			// The side only leads to b when it can be entered from d alone.
			if len(succ.Preds) != 1 || !succ.Dominates(b) {
				continue
			}
			if g, ok := s.branchGuard(d, side == 0); ok {
				out = append(out, g)
			}
		}
	}
	s.blockGuardCache[b] = out
	return out
}

// guardBranches lists the blocks that end in a branch on a parameter. Most
// blocks do not, and the guards of every block are looked for among them.
func (s *state) guardBranches() []*sg.Block {
	if s.branches != nil {
		return *s.branches
	}
	var out []*sg.Block
	for _, d := range s.fn.Blocks {
		if len(d.Succs) != 2 || d.Succs[0] == d.Succs[1] {
			continue
		}
		if _, ok := s.branchGuard(d, true); ok {
			out = append(out, d)
		}
	}
	s.branches = &out
	return out
}

// edgeGuards are the guards of the control flow edge from pred to b: those of
// pred and the branch that leads from pred to b, if there is one.
func (s *state) edgeGuards(pred, b *sg.Block) []Guard {
	if len(s.guardBranches()) == 0 {
		return nil // no condition of the function depends on a parameter
	}
	k := [2]*sg.Block{pred, b}
	if gs, ok := s.edgeGuardCache[k]; ok {
		return gs
	}
	if s.edgeGuardCache == nil {
		s.edgeGuardCache = map[[2]*sg.Block][]Guard{}
	}
	gs := s.computeEdgeGuards(pred, b)
	s.edgeGuardCache[k] = gs
	return gs
}

func (s *state) computeEdgeGuards(pred, b *sg.Block) []Guard {
	out := append([]Guard(nil), s.blockGuards(pred)...)
	if len(pred.Succs) == 2 && pred.Succs[0] != pred.Succs[1] {
		for side, succ := range pred.Succs {
			if succ == b {
				if g, ok := s.branchGuard(pred, side == 0); ok {
					out = append(out, g)
				}
			}
		}
	}
	return unionGuards(out)
}

// guardsOfChain collects the guards recorded along the reasons that led to
// the element in the cell. More than maxGuards of them is too many to be
// told apart: the flow then has none, and the caller is told.
func (s *state) guardsOfChain(l Loc, e Elem) (gs []Guard, overflow bool) {
	seen := map[ref]bool{}
	cur := ref{l, e}
	var lists [][]Guard
	for i := 0; i < maxPathSteps; i++ {
		if seen[cur] {
			break
		}
		seen[cur] = true
		w := s.whyOf(cur)
		if w == nil {
			break
		}
		if w.unguarded {
			return nil, false
		}
		if len(w.guards) > 0 {
			lists = append(lists, w.guards)
		}
		if len(w.calleeGuards) > 0 {
			lists = append(lists, s.instantiate(w.call, w.calleeGuards))
		}
		if w.from == nil {
			break
		}
		cur = *w.from
	}
	all := unionGuards(lists...)
	if len(all) > maxGuards {
		return nil, true
	}
	return all, false
}

// callInfo is what a call site contributes to the guards that come from the
// callee: the parameters get its arguments.
type callInfo struct {
	fn       *sg.Func
	instr    *sg.Instr
	pos      ir.Pos
	argv     []*sg.Value
	hasRecv  bool
	anyConst bool // some argument is a constant: the guards may be decided
}

func (s *state) newCallInfo(cc *callCtx, argv []*sg.Value) *callInfo {
	ci := &callInfo{fn: s.fn, instr: cc.in, pos: cc.in.Pos, argv: argv, hasRecv: cc.recv != nil}
	for _, a := range argv {
		if a.Kind == sg.KConst && a.Const != nil {
			ci.anyConst = true
			break
		}
	}
	return ci
}

// feasible reports whether the flow can happen through this call: it cannot
// when every parameter a guard depends on is given a constant that makes the
// condition take the other side.
func (ci *callInfo) feasible(gs []Guard) bool {
	if !ci.anyConst {
		return true
	}
	for _, g := range gs {
		if g.Call != nil {
			continue
		}
		if env, ok := constEnv(g, ci.argv); ok {
			if val, ok := evalCond(g.cond, env); ok && val != g.Polarity {
				return false
			}
		}
	}
	return true
}

// instantiate puts the call around the guards that come from the callee. It
// is done when the guards are read, not when the summary is applied: most of
// the elements applied are never asked about their guards.
func (s *state) instantiate(ci *callInfo, gs []Guard) []Guard {
	if len(gs) == 0 {
		return nil
	}
	out := make([]Guard, len(gs))
	var call *GuardCall
	for i, g := range gs {
		if g.Call == nil {
			if call == nil {
				call = &GuardCall{Caller: s.eng.funcID(ci.fn), fn: ci.fn, instr: ci.instr, pos: ci.pos}
			}
			c := *call
			for _, p := range g.Params {
				if p < len(ci.argv) {
					idx := p
					if ci.hasRecv {
						idx = p - 1
					}
					c.Args = append(c.Args, GuardArg{Param: p, ArgIndex: idx, value: ci.argv[p]})
				}
			}
			g.Call = &c
			g.k = g.computeKey()
		}
		out[i] = g
	}
	return out
}

// constEnv maps the parameters of the guard to the constants the call passes,
// when all of them are constants.
func constEnv(g Guard, argv []*sg.Value) (map[int]constant.Value, bool) {
	env := map[int]constant.Value{}
	for _, p := range g.Params {
		if p >= len(argv) {
			return nil, false
		}
		a := argv[p]
		if a.Kind != sg.KConst || a.Const == nil {
			return nil, false
		}
		cv := constValue(a.Const)
		if cv == nil {
			return nil, false
		}
		env[p] = cv
	}
	return env, true
}

// constValue turns a literal of the IR into a constant to evaluate with. A
// literal that is not an integer, a string or a boolean is not evaluated.
func constValue(c *ir.Const) constant.Value {
	switch c.Kind {
	case ir.ConstBool:
		return constant.MakeBool(c.Value == "true")
	case ir.ConstString:
		return constant.MakeString(c.Value)
	case ir.ConstInt:
		v := constant.MakeFromLiteral(c.Value, token.INT, 0)
		if v.Kind() == constant.Unknown {
			return nil
		}
		return v
	}
	return nil
}

// evalCond evaluates a condition made of parameters, constants, negation and
// comparisons. It reports false for anything else: then nothing is decided.
func evalCond(v *sg.Value, env map[int]constant.Value) (bool, bool) {
	val, ok := evalValue(v, env, 0)
	if !ok || val.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(val), true
}

var comparisons = map[string]token.Token{
	"==": token.EQL, "!=": token.NEQ, "<": token.LSS, "<=": token.LEQ, ">": token.GTR, ">=": token.GEQ,
}

func evalValue(v *sg.Value, env map[int]constant.Value, depth int) (constant.Value, bool) {
	if depth > 8 {
		return nil, false
	}
	switch v.Kind {
	case sg.KConst:
		if v.Const == nil {
			return nil, false
		}
		if cv := constValue(v.Const); cv != nil {
			return cv, true
		}
		return nil, false
	case sg.KParam:
		for i, c := range env {
			if v.Func != nil && i < len(v.Func.Params) && v.Func.Params[i] == v {
				return c, true
			}
		}
		return nil, false
	case sg.KResult:
	default:
		return nil, false
	}
	in := v.Instr
	if in == nil || in.IR == nil {
		return nil, false
	}
	switch in.Op {
	case ir.OpConvert:
		if in.IR.Operator == "convert" {
			return evalValue(in.Args[0], env, depth+1)
		}
	case ir.OpUnop:
		if in.IR.Operator == "!" {
			if b, ok := evalValue(in.Args[0], env, depth+1); ok && b.Kind() == constant.Bool {
				return constant.MakeBool(!constant.BoolVal(b)), true
			}
		}
	case ir.OpBinop:
		if op, ok := comparisons[in.IR.Operator]; ok {
			l, ok1 := evalValue(in.Args[0], env, depth+1)
			r, ok2 := evalValue(in.Args[1], env, depth+1)
			if ok1 && ok2 {
				return constant.MakeBool(constant.Compare(l, op, r)), true
			}
		}
	}
	return nil, false
}
