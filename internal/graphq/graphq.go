// Package graphq answers the questions of "rivus graph" (callers, callees,
// path) from a graph file (internal/graphdb), by the static edges and the
// dynamic ones. It does not read the source and does not run the taint
// analysis. A search asks the file for the neighbours of one function at a
// time, so it never holds the graph in memory, and it always has a bound on
// the depth and on the number of results: what it leaves out, it says.
package graphq

import (
	"fmt"
	"sort"

	"github.com/yagihash/rivus/internal/graphdb"
)

// The bounds and the thresholds. They are what the design (docs/DESIGN-graph.md,
// "Stage 2") records; a number of them was chosen from the distribution of the
// candidates of the dynamic calls of real programs.
const (
	DefaultDepth = 1
	// MaxDepth is the deepest callers / callees search.
	MaxDepth     = 6
	DefaultLimit = 50
	// ExpandCandidates: a dynamic call with at most this many candidates is
	// followed by callees; a call with more is a frontier.
	ExpandCandidates = 3
	// HubCandidates: a dynamic call with more candidates than this is not
	// used by path (it is named in hub_skipped).
	HubCandidates = 20
	DefaultPaths  = 5
	// MaxPathHops is the longest path that is searched for.
	MaxPathHops = 30
	// PathSlack is how many hops longer than the shortest a path may be.
	PathSlack = 2
	// MaxVisited bounds the functions a path search visits; MaxSteps bounds the
	// steps of the listing of the paths.
	MaxVisited = 400000
	MaxSteps   = 2000000
	// MaxFrontier is the most frontier entries a result lists.
	MaxFrontier = 50
	// MaxChoices is the most candidates a specification that names several
	// functions or calls lists (the total is said).
	MaxChoices = 20
)

// Schema is the name of the JSON the commands print.
const Schema = graphdb.SchemaName

// EdgePred says whether a search may go along an edge. nil lets every edge
// pass. --avoid and --only-impl make one (see Options).
type EdgePred func(e *graphdb.Edge) bool

// Options are what a question carries.
type Options struct {
	Depth          int
	Limit          int
	IncludeBuiltin bool
	Edge           EdgePred
	// The filters of the edges, as symbols (resolved by the engine, and named in
	// "filters" of the answer). Avoid: no edge to or from these functions. Via:
	// only what goes through them (in this order, for path). OnlyImpl: a call
	// through an interface is taken to reach only the methods of these types.
	Avoid    []string
	Via      []string
	OnlyImpl []string
	// Hubs (path): search with the calls of many candidates at once, instead of
	// only when there is no path without them.
	Hubs bool
	// FromEntry (path): the start is the entry points of the program.
	FromEntry bool

	via [][]int64
}

// Common is what every answer says.
type Common struct {
	Schema   string `json:"schema"`
	Relation string `json:"relation"`
	// Flows says that the flow of data is not what the answer speaks of: this
	// is the call graph, and no analysis of the data was asked for.
	Flows     string         `json:"flows"`
	Snapshot  any            `json:"snapshot"`
	Query     Query          `json:"query"`
	Truncated Truncated      `json:"truncated"`
	Omitted   map[string]int `json:"omitted"`
	Filters   []string       `json:"filters"`
	Hints     []string       `json:"hints"`
	Warnings  []string       `json:"warnings,omitempty"`
}

// Query is the question as it was understood.
type Query struct {
	Op             string `json:"op"`
	Target         string `json:"target,omitempty"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	Depth          int    `json:"depth,omitempty"`
	Limit          int    `json:"limit"`
	IncludeBuiltin bool   `json:"include_builtin,omitempty"`
	// Resolved says what the target was taken for.
	Resolved     string `json:"resolved,omitempty"`
	ResolvedFrom string `json:"resolved_from,omitempty"`
	ResolvedTo   string `json:"resolved_to,omitempty"`
}

// Truncated says what a limit cut: how many of Unit were left out (at least,
// when AtLeast), and the limit.
type Truncated struct {
	Count   int    `json:"count"`
	Limit   int    `json:"limit"`
	Unit    string `json:"unit"`
	AtLeast bool   `json:"at_least,omitempty"`
	// Unexpanded is the number of functions whose neighbours were not read
	// because the limit was reached.
	Unexpanded int `json:"unexpanded,omitempty"`
	// BeyondDepth is the number of functions at the last depth that have more
	// neighbours than are shown.
	BeyondDepth int `json:"beyond_depth,omitempty"`
}

// Node is a function in an answer.
type Node struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol,omitempty"`
	// At is where the function is declared.
	At     string `json:"at,omitempty"`
	Depth  int    `json:"depth"`
	NoBody bool   `json:"no_body,omitempty"`
	Kind   string `json:"kind,omitempty"`
}

// EdgeOut is a call: from a function to a function it may reach.
type EdgeOut struct {
	From string `json:"from"`
	To   string `json:"to"`
	// At is the call site.
	At   string `json:"at"`
	Kind string `json:"kind"`
	// Candidates is the number of candidates of the call site, when it is dynamic.
	Candidates int      `json:"candidates,omitempty"`
	Sites      []string `json:"sites,omitempty"`
}

// Frontier is a dynamic call where a forward search stopped.
type Frontier struct {
	From       string   `json:"from"`
	At         string   `json:"at"`
	Kind       string   `json:"kind"`
	Candidates int      `json:"candidates"`
	Callee     string   `json:"callee,omitempty"`
	Sites      []string `json:"sites,omitempty"`
}

// CallSiteOut is a call site, when it is what the target names.
type CallSiteOut struct {
	At         string   `json:"at"`
	In         string   `json:"in"`
	Kind       string   `json:"kind"`
	Candidates int      `json:"candidates"`
	Callee     string   `json:"callee,omitempty"`
	Sites      []string `json:"sites,omitempty"`
}

// Graph is the answer of callers and callees: nodes and edges.
type Graph struct {
	Common
	// Start are the functions the search began at.
	Start    []string     `json:"start"`
	CallSite *CallSiteOut `json:"call_site,omitempty"`
	Nodes    []Node       `json:"nodes"`
	Edges    []EdgeOut    `json:"edges"`
	Choices
}

// Choices is what a specification that names several things answers: the
// things (a CallSiteOut for a call, a FuncChoice for a function), up to
// MaxChoices, and how many there are.
type Choices struct {
	Ambiguous      []any `json:"ambiguous,omitempty"`
	AmbiguousTotal int   `json:"ambiguous_total,omitempty"`
}

// FuncChoice is a function a short name may stand for.
type FuncChoice struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol,omitempty"`
	// At is where it is declared.
	At     string `json:"at,omitempty"`
	NoBody bool   `json:"no_body,omitempty"`
	// Dep: a function of a dependency.
	Dep bool `json:"dep,omitempty"`
}

// CalleesResult is the answer of callees: the graph, and where it stopped.
type CalleesResult struct {
	Graph
	Frontier []Frontier `json:"frontier"`
	// FrontierTotal is the number of dynamic calls it stopped at, of which the
	// frontier lists some.
	FrontierTotal int `json:"frontier_total"`
}

// CallersResult is the answer of callers.
type CallersResult struct {
	Graph
}

// Step is one function of a path, and the call it makes to the next one.
type Step struct {
	Func   string `json:"func"`
	Symbol string `json:"symbol,omitempty"`
	// At is the call site in this function that the path goes through (none
	// at the end of a path).
	At         string `json:"at,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Candidates int    `json:"candidates,omitempty"`
	// Hub: the call has more candidates than HubCandidates.
	Hub   bool     `json:"hub,omitempty"`
	Sites []string `json:"sites,omitempty"`
	// NoBody: the function has no body in the graph (a library function that was not read).
	NoBody bool `json:"no_body,omitempty"`
	// DefinedAt is where the last function of a path is declared (the last
	// step has no call); a function with no body and no position has none.
	DefinedAt string `json:"defined_at,omitempty"`
}

// Path is a way from one function to another.
type Path struct {
	Certainty string `json:"certainty"`
	Hops      int    `json:"hops"`
	Steps     []Step `json:"steps"`
}

// EntryInfo says what the start of a path was, when it was the entry points.
type EntryInfo struct {
	// Definition says what was taken for an entry.
	Definition string `json:"definition"`
	// Source is "entry" (main, init: the extractor marked them) or "exported".
	Source   string   `json:"source"`
	Count    int      `json:"count"`
	Langs    []string `json:"langs"`
	Examples []string `json:"examples"`
}

// WithoutFilters is the answer of the same question asked again without the
// filters, when the filtered one found no path.
type WithoutFilters struct {
	Reachable bool     `json:"reachable"`
	Certainty string   `json:"certainty,omitempty"`
	Hops      int      `json:"hops,omitempty"`
	Searched  []string `json:"searched"`
}

// PathResult is the answer of path.
type PathResult struct {
	Common
	Reachable bool `json:"reachable"`
	// Certainty is that of the first path: certain (every call is named in the
	// code), uncertain (some call goes through an interface or a function value
	// with at most HubCandidates candidates), or hub (some call has more).
	Certainty string `json:"certainty,omitempty"`
	// Searched are the kinds of edges the search used: static, then dynamic (at
	// most HubCandidates candidates), then hub. A kind that is not there was not
	// searched (a path found by an earlier kind stops the search).
	Searched []string `json:"searched"`
	// Exhaustive is false when the search was cut by a bound: "no path" is then
	// not proved. When true and there is no path, it is true of what was searched.
	Exhaustive bool   `json:"exhaustive"`
	Paths      []Path `json:"paths"`
	// HubSkippedTotal is the number of calls with more than HubCandidates
	// candidates that the search met and did not go through (all of them, once
	// each); a path through one may exist. 0 when the hubs were searched, or the
	// dynamic calls were not.
	HubSkippedTotal int             `json:"hub_skipped_total,omitempty"`
	Entry           *EntryInfo      `json:"entry,omitempty"`
	WithoutFilters  *WithoutFilters `json:"without_filters,omitempty"`
	Choices
}

// Engine answers questions from a graph file.
type Engine struct {
	db      *graphdb.DB
	funcs   map[int64]*graphdb.Func
	sites   map[int64][]string
	pending []string
}

// New makes an engine over an open file.
func New(db *graphdb.DB) *Engine {
	return &Engine{db: db, funcs: map[int64]*graphdb.Func{}, sites: map[int64][]string{}}
}

func (e *Engine) fn(id int64) (*graphdb.Func, error) {
	if f, ok := e.funcs[id]; ok {
		return f, nil
	}
	f, err := e.db.FuncByID(id)
	if err != nil {
		return nil, err
	}
	e.funcs[id] = f
	return f, nil
}

func (e *Engine) siteIDs(callsite int64) ([]string, error) {
	if s, ok := e.sites[callsite]; ok {
		return s, nil
	}
	s, err := e.db.SiteIDsOfCall(callsite)
	if err != nil {
		return nil, err
	}
	e.sites[callsite] = s
	return s, nil
}

func pos(file string, line, col int) string {
	if file == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d", file, line, col)
}

func dynamic(kind string) bool { return kind == graphdb.KindInterface || kind == graphdb.KindValue }

func (e *Engine) node(id int64, depth int) (Node, error) {
	f, err := e.fn(id)
	if err != nil {
		return Node{}, err
	}
	n := Node{ID: f.FID, Depth: depth, NoBody: !f.HasBody}
	if f.Symbol != f.FID {
		n.Symbol = f.Symbol
	}
	if f.File != "" && f.Line > 0 {
		n.At = pos(f.File, f.Line, f.Col)
	}
	return n, nil
}

func (e *Engine) edgeOut(ed *graphdb.Edge) (EdgeOut, error) {
	from, err := e.fn(ed.From)
	if err != nil {
		return EdgeOut{}, err
	}
	to, err := e.fn(ed.To)
	if err != nil {
		return EdgeOut{}, err
	}
	sites, err := e.siteIDs(ed.CallSite)
	if err != nil {
		return EdgeOut{}, err
	}
	out := EdgeOut{From: from.FID, To: to.FID, At: pos(ed.File, ed.Line, ed.Col), Kind: ed.Kind, Sites: sites}
	if dynamic(ed.Kind) {
		out.Candidates = ed.NCand
	}
	return out, nil
}

func (e *Engine) callSiteOut(c *graphdb.CallRow) (CallSiteOut, error) {
	f, err := e.fn(c.Func)
	if err != nil {
		return CallSiteOut{}, err
	}
	sites, err := e.siteIDs(c.ID)
	if err != nil {
		return CallSiteOut{}, err
	}
	return CallSiteOut{At: pos(c.File, c.Line, c.Col), In: f.FID, Kind: c.Kind, Candidates: c.NCand, Callee: c.Callee, Sites: sites}, nil
}

func (e *Engine) common(op string, o Options) Common {
	return Common{Schema: Schema, Relation: "calls", Flows: "not_checked", Omitted: map[string]int{"builtin": 0}, Filters: []string{}, Hints: []string{},
		Query: Query{Op: op, Limit: o.Limit, IncludeBuiltin: o.IncludeBuiltin}}
}

func (o *Options) normalize(def int) (clamped bool) {
	if o.Depth <= 0 {
		o.Depth = DefaultDepth
	}
	if o.Depth > MaxDepth {
		o.Depth = MaxDepth
		clamped = true
	}
	if o.Limit <= 0 {
		o.Limit = def
	}
	return
}

func (o *Options) pass(ed *graphdb.Edge) bool { return o.Edge == nil || o.Edge(ed) }

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// FuncID is the ID of the function of a row (for a filter of the edges).
func (e *Engine) FuncID(id int64) (string, error) {
	f, err := e.fn(id)
	if err != nil {
		return "", err
	}
	return f.FID, nil
}
