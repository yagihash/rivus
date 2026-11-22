package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
)

// flagDeclaration is the origin of the declarations that -body-replaced makes.
const flagDeclaration = "-body-replaced"

// bodyReplacedSet is the rule set of the packages that -body-replaced names. nil when none is named.
func (c *common) bodyReplacedSet() (*model.RuleSet, error) {
	if len(c.bodyReplaced) == 0 {
		return nil, nil
	}
	rs := &model.RuleSet{Origin: flagDeclaration, DefaultOrigin: model.OriginHuman}
	seen := map[string]bool{}
	for _, p := range c.bodyReplaced {
		if err := model.CheckPackage(p); err != nil {
			return nil, fmt.Errorf("-body-replaced: %w", err)
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		rs.BodyReplaced = append(rs.BodyReplaced, model.BodyReplaced{Package: p, Reason: "named on the command line: the model is taken to stand for the body; nothing was checked"})
	}
	return rs, nil
}

// dropReplacedBodies removes from the program the functions of the dependencies in the packages that the rules
// declare, before the graph is built: the functions have no body, as they have none under -no-deps, so a call of one
// is the model when there is a row for it and unmodeled_library_call when there is none. Only the functions that
// the extractor marked as a dependency's are left out: the code of the project is always read. It returns what was
// left out, per declaration (sorted), and warns of what the command line named and that left out nothing.
func dropReplacedBodies(prog *ir.Program, rules *model.Rules, warn io.Writer) []report.BodyReplacedPackage {
	decls := rules.Declared
	if prog == nil || len(decls) == 0 {
		return nil
	}
	count := make([]int, len(decls))
	cache := map[string][]int{} // unit -> the declarations that name it
	covering := func(unit string) []int {
		if v, ok := cache[unit]; ok {
			return v
		}
		var idx []int
		if rules.ReplacedBody(unit) {
			for i, d := range decls {
				if d.Covers(unit) {
					idx = append(idx, i)
				}
			}
		}
		cache[unit] = idx
		return idx
	}
	out := prog.Funcs[:0]
	for _, f := range prog.Funcs {
		if f != nil && f.Dep {
			if idx := covering(taint.UnitOf(string(f.ID))); len(idx) > 0 {
				for _, i := range idx {
					count[i]++
				}
				continue
			}
		}
		out = append(out, f)
	}
	for i := len(out); i < len(prog.Funcs); i++ {
		prog.Funcs[i] = nil
	}
	prog.Funcs = out
	var res []report.BodyReplacedPackage
	for i, d := range decls {
		if count[i] == 0 {
			if d.Source == flagDeclaration && warn != nil {
				fmt.Fprintf(warn, "rivus: warning: -body-replaced %s: no function of a dependency is in this program (a package of the project is always read, and a package that is not used has nothing to leave out)\n", d.Package)
			}
			continue
		}
		res = append(res, report.BodyReplacedPackage{Package: d.Package, DeclaredIn: d.Source, Reason: d.Reason, Functions: count[i]})
	}
	sort.SliceStable(res, func(i, j int) bool {
		if res[i].Package != res[j].Package {
			return res[i].Package < res[j].Package
		}
		return res[i].DeclaredIn < res[j].DeclaredIn
	})
	return res
}
