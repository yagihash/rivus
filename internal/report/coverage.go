package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/taint"
)

// CoverageSchemaVersion identifies the format of Coverage.
const CoverageSchemaVersion = "rivus.coverage/0"

// SymbolCoverage is one function or interface method whose behaviour rivus
// had to guess, and how much that guess matters.
type SymbolCoverage struct {
	Symbol         string `json:"symbol"`
	Kind           string `json:"kind"`
	AttributableTo string `json:"attributable_to"`
	// Calls is the number of call sites in the analysed code.
	Calls int `json:"calls"`
	// Functions is the number of analysed functions that make such calls.
	Functions int `json:"functions"`
	// Findings is the number of findings whose path goes through such a
	// call: for them, a model decides what rivus reports. This is the
	// number to sort by.
	Findings int `json:"findings"`
	// Stub is a rule file excerpt to start from. The flows are a
	// placeholder: how taint moves through the function has to be decided by
	// a person (or a reviewed LLM answer).
	Stub string `json:"stub,omitempty"`
}

// Coverage lists the library functions and interface methods without a
// model, most useful first (ADR-015).
type Coverage struct {
	SchemaVersion string           `json:"schema_version"`
	Symbols       []SymbolCoverage `json:"symbols"`
	// NoSymbol counts uncertain calls that have no symbol, such as calls of
	// function values: no model can be written for them.
	NoSymbol int `json:"no_symbol"`
	// Total is the number of uncertainties of the kinds a model can resolve.
	Total int `json:"total"`
}

// modelable says whether writing a model resolves the uncertainty.
func modelable(u *taint.Uncertainty) bool {
	switch u.Kind {
	case taint.KindUnmodeledLibraryCall, taint.KindReflection:
		return true
	case taint.KindDynamicDispatch:
		// Only when no implementation is known: the implementations are
		// outside the loaded code, so a model for the interface method
		// stands in for them.
		return len(u.Candidates) == 0 && !strings.HasPrefix(u.Detail, "expansion_limit")
	}
	return false
}

// Coverage computes the model coverage of the analysed code.
func (r *Report) Coverage() *Coverage {
	type key struct{ kind, symbol string }
	type acc struct {
		sym   SymbolCoverage
		funcs map[string]bool
	}
	by := map[key]*acc{}
	out := &Coverage{SchemaVersion: CoverageSchemaVersion, Symbols: []SymbolCoverage{}}
	for _, u := range r.Uncertainties {
		if !modelable(u) {
			continue
		}
		out.Total++
		if u.Symbol == "" {
			out.NoSymbol++
			continue
		}
		k := key{u.Kind, u.Symbol}
		a := by[k]
		if a == nil {
			a = &acc{sym: SymbolCoverage{Symbol: u.Symbol, Kind: u.Kind, AttributableTo: u.AttributableTo}, funcs: map[string]bool{}}
			by[k] = a
		}
		a.sym.Calls++
		a.funcs[u.Func] = true
	}
	for _, f := range r.Findings {
		seen := map[key]bool{}
		for _, u := range f.Uncertainties {
			k := key{u.Kind, u.Symbol}
			if a := by[k]; a != nil && !seen[k] {
				seen[k] = true
				a.sym.Findings++
			}
		}
	}
	for _, a := range by {
		a.sym.Functions = len(a.funcs)
		a.sym.Stub = stub(a.sym.Symbol)
		out.Symbols = append(out.Symbols, a.sym)
	}
	sort.Slice(out.Symbols, func(i, j int) bool {
		a, b := out.Symbols[i], out.Symbols[j]
		if a.Findings != b.Findings {
			return a.Findings > b.Findings
		}
		if a.Calls != b.Calls {
			return a.Calls > b.Calls
		}
		return a.Symbol < b.Symbol
	})
	return out
}

// stub is the start of a propagator rule for the symbol. A rule replaces what the analysis does for a call that has
// no model: the receiver and the arguments flow to the result, and the memory the arguments point to is read as
// unknown (a function that is not read may write into what it is given). The stub keeps the first part: the receiver
// and the arguments flow to `results`, every result (`result` is the first result only, so a function that returns
// `(value, error)` would lose the flow into the error). The writes into the arguments are a comment: `args` and `recv`
// as a destination write into every argument, and a shared object (a mux, a client, a configuration) then holds the
// taint of every call (the findings of a project went up about 8 times). Name the arguments that are written
// (`arg1`) and take the comment away. TestModelsCoverageStubsLoseNoFinding says what that costs on bench/gcp.
func stub(symbol string) string {
	return fmt.Sprintf("  - symbol: %q\n    flows: [{from: recv, to: results}, {from: args, to: results}]  # TODO: check how taint moves; `results` is every result (`result` is the first only)\n    # writes into the arguments, off by default (the memory the arguments point to is then not read as unknown): name the destination (`to: arg1`), not `args`; a write into a shared object (mux, client, configuration) adds findings\n    # flows: [{from: recv, to: results}, {from: args, to: results}, {from: recv, to: arg1}, {from: args, to: arg1}]\n", symbol)
}

// YAML renders the first n symbols as a rule file to fill in.
func (c *Coverage) YAML(n int) string {
	var b strings.Builder
	b.WriteString("# Generated by \"rivus models coverage -yaml\", most useful first.\n")
	b.WriteString("# Every flow below is a placeholder: decide how taint moves through the\n")
	b.WriteString("# function, then remove the TODO. Add origin: human once it is reviewed.\n")
	b.WriteString("ruleset: myorg/models@1\npropagators:\n")
	for i, s := range c.Symbols {
		if n > 0 && i >= n {
			break
		}
		fmt.Fprintf(&b, "  # %s: %d calls, %d findings\n%s", s.Kind, s.Calls, s.Findings, s.Stub)
	}
	return b.String()
}
