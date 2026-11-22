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
		shape shape
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
		a.shape.add(u)
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
		a.sym.Stub = stub(a.sym.Symbol, &a.shape)
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

// shape is what the calls of a symbol say about the operands: the receiver, and the arguments by position.
type shape struct {
	known   bool // a call said the types of its operands
	recv    bool
	recvPtr bool
	args    []operandShape
}

type operandShape struct {
	writable bool // a pointer, a slice, a map, an interface: something a function can write into
	context  bool // every call gave a context.Context here
	seen     int
	ctxSeen  int
	class    string
}

// contextType is the type of an argument that never carries data and is never written to.
const contextType = "context.Context"

func (sh *shape) add(u *taint.Uncertainty) {
	if u.Recv == nil && len(u.Operands) == 0 {
		return
	}
	sh.known = true
	if u.Recv != nil {
		sh.recv = true
		sh.recvPtr = sh.recvPtr || u.Recv.Class == "pointer" || u.Recv.Class == "interface" || u.Recv.Class == "container"
	}
	for len(sh.args) < len(u.Operands) {
		sh.args = append(sh.args, operandShape{})
	}
	for i, o := range u.Operands {
		a := &sh.args[i]
		a.seen++
		if string(o.Type) == contextType {
			a.ctxSeen++
			continue
		}
		switch o.Class {
		case "pointer", "container", "interface", "other", "":
			a.writable = true
			if a.class == "" || a.class == "other" {
				a.class = string(o.Class)
			}
		}
	}
}

// stub is the start of a propagator rule for the symbol. A rule replaces what the analysis does for a call that has
// no model: the receiver and the arguments flow to the result, and the memory the arguments point to is read as
// unknown (a function that is not read may write into what it is given). The stub keeps both: the receiver and the
// arguments flow to `results`, every result (`result` is the first result only, so a function that returns
// `(value, error)` would lose the flow into the error), and into each argument that can be written into (a pointer, a
// slice, a map, an interface: from the type of the operand at the calls; a string or a number cannot) and into the
// receiver when it is a pointer, an interface, a slice or a map, so that putting every stub in loses no finding (it can add some). A
// `context.Context` is left out of the writes (it stays a source of the results). A row that names an argument says in a comment to delete it when the
// function does not write there: a shared object (a mux, a client, a configuration) that is kept adds findings.
// TestModelsCoverageStubsLoseNoFinding: the first stub left the receiver out, and then left the writes into the
// arguments out; 3 and 2 of the 27 findings of bench/gcp went.
func stub(symbol string, sh *shape) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  - symbol: %q\n    flows:  # TODO: check how taint moves; `results` is every result (`result` is the first only)\n", symbol)
	if !sh.known {
		// the types of the operands are not known (an index of an older version): the wide row, which loses nothing
		b.WriteString("      - {from: recv, to: results}\n      - {from: args, to: results}\n")
		b.WriteString("      # types unknown: this writes into every argument; name the ones the function writes into (to: arg1) and delete this\n")
		b.WriteString("      - {from: recv, to: args}\n      - {from: args, to: args}\n")
		return b.String()
	}
	// the arguments that carry data into a write: all of them but the contexts
	var srcs []string
	ctx := false
	for i, a := range sh.args {
		if a.ctxSeen == a.seen && a.seen > 0 {
			ctx = true
			continue
		}
		srcs = append(srcs, fmt.Sprintf("arg%d", i))
	}
	if !ctx {
		srcs = []string{"args"}
	}
	flow := func(from, to, note string) {
		fmt.Fprintf(&b, "      - {from: %s, to: %s}%s\n", from, to, note)
	}
	if sh.recv {
		flow("recv", "results", "")
	}
	// The context stays a source of the results: a context made from the request is how the taint of a request
	// reaches what a call of a client gives back (bench/gcp: 3 of the 27 findings go through `r.Context()`), and
	// leaving it out loses them. It is left out of the writes, where it only spreads the taint to every holder of it.
	flow("args", "results", "")
	for i, a := range sh.args {
		if !a.writable || (a.ctxSeen == a.seen && a.seen > 0) {
			continue
		}
		fmt.Fprintf(&b, "      # writes into arg%d (%s): delete these lines if the function does not write into it; a shared object (router, client, configuration) kept here adds many findings\n", i, orNone(a.class))
		if sh.recv {
			flow("recv", fmt.Sprintf("arg%d", i), "")
		}
		for _, f := range srcs {
			flow(f, fmt.Sprintf("arg%d", i), "")
		}
	}
	if sh.recv && sh.recvPtr {
		b.WriteString("      # writes into the receiver (a pointer, an interface, a slice or a map: a value receiver is a copy and is not written): delete these lines if the function does not write into it\n")
		for _, f := range srcs {
			flow(f, "recv", "")
		}
	}
	return b.String()
}

func orNone(c string) string {
	if c == "" {
		return "type unknown"
	}
	return c
}

// YAML renders the first n symbols as a rule file to fill in.
func (c *Coverage) YAML(n int) string {
	var b strings.Builder
	b.WriteString("# Generated by \"rivus models coverage -yaml\", most useful first.\n")
	b.WriteString("# Every flow below is a placeholder: decide how taint moves through the\n")
	b.WriteString("# function, then remove the TODO. Add origin: human once it is reviewed.\n")
	b.WriteString("# A function that registers a handler (a router) and hands it what it took from the request: its parameters are\n")
	b.WriteString("# a source, not a flow. Add a source like this one (the position of the handler among the operands, the first\n")
	b.WriteString("# parameter that is input), and keep the flows below for what the function does with its arguments:\n")
	b.WriteString("#   sources:\n#     - {kind: callback_param, symbol: \"<the registering function>\", operand: 1, from: 1, labels: [user_input]}\n")
	b.WriteString("# A function that calls a function it is given (`calls:`, docs/MODELING.md section 3): in a built-in model, the row of a package\n")
	b.WriteString("# (not the standard library) says `checked_by: cell`, and a test cell that goes through it is the check.\n")
	b.WriteString("ruleset: myorg/models@1\npropagators:\n")
	for i, s := range c.Symbols {
		if n > 0 && i >= n {
			break
		}
		fmt.Fprintf(&b, "  # %s: %d calls, %d findings\n%s", s.Kind, s.Calls, s.Findings, s.Stub)
	}
	return b.String()
}
