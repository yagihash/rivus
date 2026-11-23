package accept

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
)

// Report is every difference between two programs.
type Report struct {
	// OnlyA and OnlyB are the IDs of the functions that only one side has.
	OnlyA, OnlyB []ir.FuncID
	// FuncDiffs are the differences of the body of each function both sides have (DiffAll), by ID.
	FuncDiffs map[ir.FuncID][]string
	// CandDiffs are the differences of the candidates of the calls, one line for each call site.
	CandDiffs []string
	// ProgramDiffs are the differences in the rest of the program (globals, implementations, types, texts, ...).
	ProgramDiffs []string
}

// Empty says there is no difference.
func (r Report) Empty() bool {
	return len(r.OnlyA) == 0 && len(r.OnlyB) == 0 && len(r.FuncDiffs) == 0 && len(r.CandDiffs) == 0 && len(r.ProgramDiffs) == 0
}

// Compare finds every difference between two programs. The order of Funcs does not count (the functions are
// matched by ID); nothing else is left out or treated as the same, except what DiffAll treats as the same.
func Compare(a, b *ir.Program) Report {
	r := Report{FuncDiffs: map[ir.FuncID][]string{}}
	fa, fb := byID(a), byID(b)
	for id, f := range fa {
		g, ok := fb[id]
		if !ok {
			r.OnlyA = append(r.OnlyA, id)
			continue
		}
		if d := DiffAll(f, g); len(d) != 0 {
			r.FuncDiffs[id] = d
		}
		r.CandDiffs = append(r.CandDiffs, candDiffs(f, g)...)
	}
	for id := range fb {
		if _, ok := fa[id]; !ok {
			r.OnlyB = append(r.OnlyB, id)
		}
	}
	sort.Slice(r.OnlyA, func(i, j int) bool { return r.OnlyA[i] < r.OnlyA[j] })
	sort.Slice(r.OnlyB, func(i, j int) bool { return r.OnlyB[i] < r.OnlyB[j] })
	sort.Strings(r.CandDiffs)

	pa, pb := reflect.ValueOf(a).Elem(), reflect.ValueOf(b).Elem()
	t := pa.Type()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("json") == "-" || t.Field(i).Name == "Funcs" {
			continue
		}
		r.ProgramDiffs = diffValue(pa.Field(i), pb.Field(i), "Program."+t.Field(i).Name, r.ProgramDiffs)
	}
	return r
}

func byID(p *ir.Program) map[ir.FuncID]*ir.Func {
	m := make(map[ir.FuncID]*ir.Func, len(p.Funcs))
	for _, f := range p.Funcs {
		m[f.ID] = f
	}
	return m
}

// candDiffs lists, for each call of two copies of a function, the candidates one side has and the other lacks.
// The same differences are in DiffAll; this view keeps them apart, so that a function with many is seen as such.
func candDiffs(a, b *ir.Func) []string {
	var out []string
	for bi := 0; bi < len(a.Blocks) && bi < len(b.Blocks); bi++ {
		ia, ib := a.Blocks[bi].Instrs, b.Blocks[bi].Instrs
		for ii := 0; ii < len(ia) && ii < len(ib); ii++ {
			ca, cb := ia[ii].Callee, ib[ii].Callee
			if ca == nil || cb == nil {
				continue
			}
			sa, sb := set(ca.Candidates), set(cb.Candidates)
			var lack, extra []string
			for id := range sa {
				if !sb[id] {
					lack = append(lack, string(id))
				}
			}
			for id := range sb {
				if !sa[id] {
					extra = append(extra, string(id))
				}
			}
			if len(lack)+len(extra) == 0 {
				continue
			}
			sort.Strings(lack)
			sort.Strings(extra)
			out = append(out, fmt.Sprintf("%s block %d instr %d: only first [%s], only second [%s]",
				a.ID, bi, ii, strings.Join(lack, " "), strings.Join(extra, " ")))
		}
	}
	return out
}

func set(ids []ir.FuncID) map[ir.FuncID]bool {
	m := make(map[ir.FuncID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// Summary writes the report: the counts first, then the first limit of each kind (all of them when limit <= 0).
func (r Report) Summary(limit int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "functions only in the first: %d; only in the second: %d; functions with a different body: %d; calls with different candidates: %d; other differences: %d\n",
		len(r.OnlyA), len(r.OnlyB), len(r.FuncDiffs), len(r.CandDiffs), len(r.ProgramDiffs))
	cut := func(n int) int {
		if limit > 0 && n > limit {
			return limit
		}
		return n
	}
	for _, id := range r.OnlyA[:cut(len(r.OnlyA))] {
		fmt.Fprintf(&b, "  only in the first: %s\n", id)
	}
	for _, id := range r.OnlyB[:cut(len(r.OnlyB))] {
		fmt.Fprintf(&b, "  only in the second: %s\n", id)
	}
	ids := make([]ir.FuncID, 0, len(r.FuncDiffs))
	for id := range r.FuncDiffs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids[:cut(len(ids))] {
		d := r.FuncDiffs[id]
		fmt.Fprintf(&b, "  %s: %d differences\n", id, len(d))
		for _, s := range d[:min(3, len(d))] {
			fmt.Fprintf(&b, "    %s\n", s)
		}
	}
	for _, s := range r.CandDiffs[:cut(len(r.CandDiffs))] {
		fmt.Fprintf(&b, "  candidates: %s\n", s)
	}
	for _, s := range r.ProgramDiffs[:cut(len(r.ProgramDiffs))] {
		fmt.Fprintf(&b, "  %s\n", s)
	}
	return b.String()
}
