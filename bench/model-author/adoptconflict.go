package main

import (
	"fmt"
	"sort"
	"strings"
)

// The conflict policy of bulk adoption (docs/DESIGN-package-models.md, 6.12, 1 step 4). Two decisions of `adopt_rules:` that claim the same
// position of the same symbol:
//   - the same category: the claims are merged into one row, and a warning names both decisions;
//   - different categories: a human decides (`categories` for both, or the decision narrowed): the command stops with ExitUndecided.
// ResolveClaims is the detection, a pure function of the claims; the expansion that calls it is a later step.

// Claim is one position of one symbol that a decision wants as a row.
type Claim struct {
	Decision string
	Symbol   string
	Position string // arg0, arg1.Field, field, result
	Category string // sink only; "" for a source
}

// Merged is a position with its claims joined: one row. Decisions are all the ids, in dictionary order without duplicates: the row writes them
// all in `decision:` (a list when there are several).
type Merged struct {
	Symbol    string
	Position  string
	Category  string
	Decisions []string
}

// Conflict is a position that decisions of different categories claim.
type Conflict struct {
	Symbol     string
	Position   string
	Decisions  []string
	Categories []string
}

func (c Conflict) String() string {
	return fmt.Sprintf("%s %s: decisions %s claim categories %s: a human decides (`categories` for both, or one of the decisions narrowed)",
		c.Symbol, c.Position, strings.Join(c.Decisions, ", "), strings.Join(c.Categories, ", "))
}

// ConflictError is the error of ResolveClaims; the command exits with ExitUndecided on it.
type ConflictError struct{ Conflicts []Conflict }

func (e *ConflictError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d position(s) are claimed with different categories by two decisions (exit %d):", len(e.Conflicts), ExitUndecided)
	for _, c := range e.Conflicts {
		b.WriteString("\n  " + c.String())
	}
	return b.String()
}

// ResolveClaims joins the claims by (symbol, position). The result is sorted by symbol and position, whatever the order of the claims. The same
// decision claiming a position twice counts once. The warnings name both decisions of a merge. With a conflict, the error lists all of them (and the
// merged rows are those without a conflict).
func ResolveClaims(claims []Claim) (merged []Merged, warnings []string, err error) {
	type key struct{ sym, pos string }
	order := []key{}
	by := map[key][]Claim{}
	for _, c := range claims {
		k := key{c.Symbol, c.Position}
		if _, ok := by[k]; !ok {
			order = append(order, k)
		}
		by[k] = append(by[k], c)
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].sym != order[j].sym {
			return order[i].sym < order[j].sym
		}
		return order[i].pos < order[j].pos
	})
	var conflicts []Conflict
	for _, k := range order {
		var decisions, cats []string
		for _, c := range by[k] {
			decisions = appendOnce(decisions, c.Decision)
			cats = appendOnce(cats, c.Category)
		}
		sort.Strings(decisions)
		if len(cats) > 1 {
			sort.Strings(cats)
			conflicts = append(conflicts, Conflict{Symbol: k.sym, Position: k.pos, Decisions: decisions, Categories: cats})
			continue
		}
		if len(decisions) > 1 {
			warnings = append(warnings, fmt.Sprintf("%s %s: decisions %s claim it with the same category: merged into one row", k.sym, k.pos, strings.Join(decisions, " and ")))
		}
		merged = append(merged, Merged{Symbol: k.sym, Position: k.pos, Category: cats[0], Decisions: decisions})
	}
	if len(conflicts) > 0 {
		return merged, warnings, &ConflictError{Conflicts: conflicts}
	}
	return merged, warnings, nil
}

func appendOnce(xs []string, x string) []string {
	for _, y := range xs {
		if y == x {
			return xs
		}
	}
	return append(xs, x)
}
