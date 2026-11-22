package model

import (
	"fmt"
	"strings"
)

// Rejected is a candidate of a model that was looked at and not adopted (docs/DESIGN-package-models.md, 6.11; docs/MODELING.md,
// "Rejected candidates"). It is a record: the analysis does not read it, and it changes no row. The command `make model` lists the
// candidates of a package and matches each of them with a row or with an entry here; one that is neither is undecided.
type Rejected struct {
	// Symbol is the symbol of the function or method, written as in the rows.
	Symbol string `yaml:"symbol" json:"symbol"`
	// Kind is what the candidate would have been: sink, source, sanitizer or calls.
	Kind string `yaml:"kind" json:"kind"`
	// Why is the reason, in a few words.
	Why string `yaml:"why" json:"why"`
	// Position is the argument or result that was looked at (arg0, recv, result, ...), when the reason is about one place.
	Position string `yaml:"position" json:"position,omitempty"`
}

// RejectedKinds are the values of kind.
var RejectedKinds = []string{"sink", "source", "sanitizer", "calls"}

func (rs *RuleSet) validateRejected() error {
	seen := map[string]bool{}
	for i, r := range rs.Rejected {
		what := fmt.Sprintf("rejected[%d]", i)
		if strings.TrimSpace(r.Symbol) == "" {
			return fmt.Errorf("%s: symbol is required", what)
		}
		if err := checkSymbol(r.Symbol, what); err != nil {
			return err
		}
		ok := false
		for _, k := range RejectedKinds {
			ok = ok || r.Kind == k
		}
		if r.Kind == "" {
			return fmt.Errorf("%s (%s): kind is required (%s)", what, r.Symbol, strings.Join(RejectedKinds, ", "))
		}
		if !ok {
			return fmt.Errorf("%s (%s): unknown kind %q (%s)", what, r.Symbol, r.Kind, strings.Join(RejectedKinds, ", "))
		}
		if strings.TrimSpace(r.Why) == "" {
			return fmt.Errorf("%s (%s): why is required", what, r.Symbol)
		}
		key := r.Symbol + "\x00" + r.Kind + "\x00" + r.Position
		if seen[key] {
			return fmt.Errorf("%s (%s): the same symbol, kind and position are rejected twice", what, r.Symbol)
		}
		seen[key] = true
	}
	return nil
}
