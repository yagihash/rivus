package ir

import (
	"fmt"
	"strconv"
	"strings"
)

// Format renders a function as text a person can read: the check that an
// extractor does what it should is a first class use of the IR.
func (f *Func) Format() string {
	var sb strings.Builder
	var params []string
	for i, p := range f.Params {
		s := fmt.Sprintf("%d:%s", i, orDash(p.Name))
		if p.Captured {
			s += "(captured)"
		}
		params = append(params, s)
	}
	fmt.Fprintf(&sb, "func %s(%s)", f.ID, strings.Join(params, ", "))
	if len(f.Results) > 0 {
		var rs []string
		for _, r := range f.Results {
			rs = append(rs, string(r))
		}
		fmt.Fprintf(&sb, " -> (%s)", strings.Join(rs, ", "))
	}
	sb.WriteString("\n")
	roots := map[BlockID]bool{}
	for _, r := range f.Roots {
		roots[r] = true
	}
	for _, b := range f.Blocks {
		fmt.Fprintf(&sb, "  b%d:", b.ID)
		if roots[b.ID] && b.ID != 0 {
			sb.WriteString("  ; root")
		}
		sb.WriteString("\n")
		for _, in := range b.Instrs {
			fmt.Fprintf(&sb, "    %s\n", f.formatInstr(in))
		}
		fmt.Fprintf(&sb, "    %s\n", f.formatTerm(b))
	}
	return sb.String()
}

func orDash(s string) string {
	if s == "" {
		return "_"
	}
	return s
}

func (f *Func) name(id LocalID) string {
	if id < 0 || int(id) >= len(f.Locals) {
		return fmt.Sprintf("%%?%d", id)
	}
	if n := f.Locals[id].Name; n != "" {
		return fmt.Sprintf("%%%d(%s)", id, n)
	}
	return fmt.Sprintf("%%%d", id)
}

func (f *Func) names(ids []LocalID) string {
	var out []string
	for _, id := range ids {
		out = append(out, f.name(id))
	}
	return strings.Join(out, ", ")
}

func (f *Func) formatInstr(in Instr) string {
	lhs := ""
	if len(in.Dsts) > 0 {
		lhs = f.names(in.Dsts) + " = "
	}
	args := f.names(in.Args)
	var body string
	switch in.Op {
	case OpConst:
		body = "const " + string(in.Const.Kind)
		if in.Const.Kind != ConstNil {
			body += " " + strconv.Quote(in.Const.Value)
		}
		if in.Const.Truncated {
			body += " (truncated)"
		}
	case OpParam:
		body = fmt.Sprintf("param %d", in.Index)
	case OpCopy:
		body = "copy " + args
		if in.View {
			body += " (view)"
		}
	case OpUnop, OpBinop:
		body = fmt.Sprintf("%s %q %s", in.Op, in.Operator, args)
	case OpConvert, OpTypeAssert, OpAlloc:
		body = fmt.Sprintf("%s %s", in.Op, in.Type)
		if args != "" {
			body += " " + args
		}
	case OpAddrOf:
		var ps []string
		for _, s := range in.Path {
			switch s.Kind {
			case StepField:
				ps = append(ps, "."+s.Field)
			case StepIndex:
				ps = append(ps, "[]")
			case StepDeref:
				ps = append(ps, "*")
			}
		}
		body = "addr_of " + args + strings.Join(ps, "")
	case OpFieldLoad, OpFieldStore:
		body = fmt.Sprintf("%s .%s %s", in.Op, in.Field, args)
	case OpGlobalLoad, OpGlobalStore:
		body = fmt.Sprintf("%s %s %s", in.Op, in.Global, args)
	case OpMakeClosure:
		body = fmt.Sprintf("make_closure %s [%s]", in.Func, args)
	case OpCall:
		var fl []string
		if in.Go {
			fl = append(fl, "go")
		}
		if in.Defer {
			fl = append(fl, "defer")
		}
		if in.Await {
			fl = append(fl, "await")
		}
		body = fmt.Sprintf("call %s %s(%s)", in.Callee.Kind, in.Callee.Symbol, args)
		if len(in.Callee.Candidates) > 0 {
			body += fmt.Sprintf(" candidates=%d", len(in.Callee.Candidates))
		}
		if len(fl) > 0 {
			body += " [" + strings.Join(fl, ",") + "]"
		}
	case OpUnsupported:
		body = "unsupported " + strconv.Quote(in.Reason)
	default:
		body = string(in.Op)
		if args != "" {
			body += " " + args
		}
	}
	return lhs + strings.TrimRight(body, " ")
}

func (f *Func) formatTerm(b Block) string {
	var succ []string
	for _, e := range b.Succs {
		succ = append(succ, fmt.Sprintf("%s->b%d", e.Kind, e.To))
	}
	t := b.Term
	out := string(t.Kind)
	switch t.Kind {
	case TermBranch:
		out += " " + f.name(t.Cond)
	case TermReturn, TermThrow:
		if len(t.Values) > 0 {
			out += " " + f.names(t.Values)
		}
	}
	if len(succ) > 0 {
		out += "  ; " + strings.Join(succ, " ")
	}
	return out
}
