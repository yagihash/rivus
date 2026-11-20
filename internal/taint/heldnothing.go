package taint

import (
	"sort"

	"github.com/yagihash/rivus/internal/sg"
)

// KindGlobalHoldsNothing marks a package variable that a rule says holds nothing (ADR-056): what the
// functions listed wrote to it is not followed. Attributable to rivus: it is a decision of the model, and a
// rule file takes it back (holds: everything).
const KindGlobalHoldsNothing = "global_holds_nothing"

// DroppedGlobal is a package variable that holds nothing, and the number of functions that wrote to it.
type DroppedGlobal struct {
	Symbol    string `json:"symbol"`
	Functions int    `json:"functions"`
}

func (e *Engine) noteHeldNothing(name string, fn *sg.Func) {
	e.heldMu.Lock()
	defer e.heldMu.Unlock()
	if e.heldNothing == nil {
		e.heldNothing = map[string]map[*sg.Func]bool{}
	}
	if e.heldNothing[name] == nil {
		e.heldNothing[name] = map[*sg.Func]bool{}
	}
	e.heldNothing[name][fn] = true
}

func (e *Engine) droppedGlobals() []DroppedGlobal {
	e.heldMu.Lock()
	defer e.heldMu.Unlock()
	var out []DroppedGlobal
	for name, fns := range e.heldNothing {
		out = append(out, DroppedGlobal{name, len(fns)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

// heldNothingRecords is one uncertainty for each package variable that holds nothing and was written to.
func (e *Engine) heldNothingRecords() []*Uncertainty {
	e.heldMu.Lock()
	defer e.heldMu.Unlock()
	names := make([]string, 0, len(e.heldNothing))
	for name := range e.heldNothing {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []*Uncertainty
	for _, name := range names {
		fns := make([]*sg.Func, 0, len(e.heldNothing[name]))
		for fn := range e.heldNothing[name] {
			fns = append(fns, fn)
		}
		sort.Slice(fns, func(i, j int) bool {
			if fns[i].Dep != fns[j].Dep {
				return !fns[i].Dep // the code of the project first
			}
			return fns[i].ID < fns[j].ID
		})
		u := newUncertainty(KindGlobalHoldsNothing, "global_holds_nothing: "+itoa(len(fns))+" function(s) wrote data that rivus follows to "+name+
			", which a rule says holds nothing (what is written to it goes out of the program): the data is not followed through it. "+
			"A program that reads it back (os.Pipe, a replaced os.Stdout) is not followed; \"holds: everything\" in a rule file follows it", name)
		u.Func = fns[0].ID
		u.pos = fns[0].Pos
		for i, fn := range fns {
			if i >= 10 {
				break
			}
			u.Candidates = append(u.Candidates, fn.ID)
		}
		e.fill(u)
		out = append(out, u)
	}
	return out
}
