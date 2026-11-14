package irtaint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/sg"
)

// conditionalNodes turns the guards of a finding into nodes. A guard is
// worth a node only if the function does different things on its two sides:
// otherwise the condition (an early return, a loop test) says nothing about
// the flow.
func (e *Engine) conditionalNodes(guards []Guard) []ConditionalNode {
	// One node per condition; prefer the instance that knows its call site.
	byCond := map[string]Guard{}
	var order []string
	for _, g := range guards {
		k := g.condKey()
		cur, ok := byCond[k]
		if !ok {
			order = append(order, k)
			byCond[k] = g
		} else if cur.Call == nil && g.Call != nil {
			byCond[k] = g
		}
	}
	var out []ConditionalNode
	for _, k := range order {
		g := byCond[k]
		outcomes := e.outcomesOf(g)
		if len(outcomes[true]) == 0 || len(outcomes[false]) == 0 || sameSet(outcomes[true], outcomes[false]) {
			continue
		}
		out = append(out, e.node(g, outcomes))
	}
	return out
}

// outcomesOf describes, for each side of the condition, what the function
// does with taint, from its summary.
func (e *Engine) outcomesOf(g Guard) map[bool]map[string]bool {
	res := map[bool]map[string]bool{true: {}, false: {}}
	sum := e.summaries[g.fn]
	if sum == nil {
		return res
	}
	for _, o := range sum.Outputs {
		for _, oe := range o.Elems {
			for _, og := range oe.Guards {
				if og.condKey() == g.condKey() {
					res[og.Polarity][describe(oe)] = true
				}
			}
		}
	}
	return res
}

// describe says in a few words what an output element stands for.
func describe(oe OutElem) string {
	var by string
	oe.Steps.each(func(st Step) bool {
		if st.Model != nil && st.Model.Role == "sanitizer" {
			by = st.Model.Symbol
			return false
		}
		return true
	})
	switch {
	case by != "":
		return "sanitized by " + by
	case oe.E.Kind == Param && oe.E.Mods != "":
		var drops []string
		for _, o := range decodeOps(oe.E.Mods) {
			drops = append(drops, o.drops...)
		}
		return "labels dropped: " + strings.Join(drops, ", ")
	case oe.E.Kind == Param:
		return "tainted (passthrough)"
	}
	return "tainted (" + oe.E.Label + ")"
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func joinSet(m map[string]bool) string {
	list := make([]string, 0, len(m))
	for k := range m {
		list = append(list, k)
	}
	sort.Strings(list)
	return strings.Join(list, "; ")
}

func (e *Engine) node(g Guard, outcomes map[bool]map[string]bool) ConditionalNode {
	n := ConditionalNode{
		Function:        g.Func,
		Location:        position(g.pos),
		BranchCondition: g.Cond,
		Outcomes: map[string]string{
			g.Cond + " == true":  joinSet(outcomes[true]),
			g.Cond + " == false": joinSet(outcomes[false]),
		},
		FunctionSource: e.texts.FuncSource(g.fn.IR.ID, 80),
	}
	n.Kind = CondFlow
	for _, side := range []bool{true, false} {
		for d := range outcomes[side] {
			if strings.HasPrefix(d, "sanitized by") || strings.HasPrefix(d, "labels dropped") {
				n.Kind = CondSanitization
			}
		}
	}
	var labels []string
	for _, p := range g.Params {
		labels = append(labels, paramLabel(g.fn, p))
	}
	n.DerivedFrom = strings.Join(labels, ", ")

	if g.Call != nil {
		pos := position(g.Call.pos)
		n.CallSite = &pos
		var args []string
		for _, a := range g.Call.Args {
			if a.ArgIndex < 0 {
				continue
			}
			arg, call := e.texts.CallArg(g.Call.pos, a.ArgIndex)
			n.CallSiteArgument = call
			args = append(args, arg)
		}
		n.Argument = strings.Join(args, ", ")
		n.Resolved = len(g.Call.Args) > 0
		for _, a := range g.Call.Args {
			if a.value.Kind != sg.KConst {
				n.Resolved = false
			}
		}
	}
	n.Provenance = e.provenanceOf(g)
	return n
}

var _ = fmt.Sprintf

// paramLabel describes a parameter of a function for a reader, such as
// "parameter 1 (escape bool)". Parameters count from 0, the receiver first.
func paramLabel(fn *sg.Func, i int) string {
	if i >= 0 && i < len(fn.IR.Params) {
		p := fn.IR.Params[i]
		return fmt.Sprintf("parameter %d (%s %s)", i, p.Name, p.Display)
	}
	return fmt.Sprintf("parameter %d", i)
}
