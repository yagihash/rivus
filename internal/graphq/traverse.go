package graphq

import (
	"fmt"
	"strings"

	"github.com/yagihash/rivus/internal/graphdb"
)

// traversal is the state of a breadth-first search over callers or callees.
type traversal struct {
	e      *Engine
	o      *Options
	g      *Graph
	depth  map[int64]int
	nEdges int
	// cut is true once the limit of edges is reached.
	cut bool
}

func (t *traversal) addNode(id int64, d int) error {
	if _, ok := t.depth[id]; ok {
		return nil
	}
	t.depth[id] = d
	n, err := t.e.node(id, d)
	if err != nil {
		return err
	}
	t.g.Nodes = append(t.g.Nodes, n)
	return nil
}

// emit adds an edge; it reports false when the limit is reached (the edge is
// then counted as cut).
func (t *traversal) emit(ed *graphdb.Edge) (bool, error) {
	if t.nEdges >= t.o.Limit {
		t.cut = true
		t.g.Truncated.Count++
		return false, nil
	}
	out, err := t.e.edgeOut(ed)
	if err != nil {
		return false, err
	}
	t.g.Edges = append(t.g.Edges, out)
	t.nEdges++
	return true, nil
}

func (e *Engine) newGraph(op string, o Options, target string, tg *Target, unit string) (*Graph, error) {
	g := &Graph{Common: e.common(op, o), Nodes: []Node{}, Edges: []EdgeOut{}, Start: []string{}}
	g.Query.Target = target
	g.Query.Depth = o.Depth
	g.Query.Resolved = tg.Desc
	g.Truncated = Truncated{Limit: o.Limit, Unit: unit}
	if tg.Builtin > 0 {
		g.Omitted["builtin"] += tg.Builtin
	}
	return g, nil
}

func (e *Engine) ambiguousResult(g *Graph, tg *Target) error {
	amb, err := e.ambiguous(tg.Ambiguous)
	if err != nil {
		return err
	}
	g.Ambiguous = amb
	g.Hints = append(g.Hints, "several calls on the line: ask again with one of the \"at\" values (file:line:col), or --site <id>")
	return nil
}

// Callers answers who calls the function a target names (or that holds the
// call a position names). Every kind of edge is followed; a dynamic one carries
// its kind and the number of candidates of its call site: "the caller may call
// this function, as one of N".
func (e *Engine) Callers(target string, o Options) (*CallersResult, error) {
	clamped := o.normalize(DefaultLimit)
	tg, err := e.Resolve(target, o.IncludeBuiltin)
	if err != nil {
		return nil, err
	}
	g, _ := e.newGraph("callers", o, target, tg, "edges")
	res := &CallersResult{Graph: *g}
	g = &res.Graph
	if clamped {
		g.Warnings = append(g.Warnings, fmt.Sprintf("--depth is limited to %d", MaxDepth))
	}
	if tg.Ambiguous != nil {
		return res, e.ambiguousResult(g, tg)
	}
	if tg.Call != nil {
		cs, err := e.callSiteOut(tg.Call)
		if err != nil {
			return nil, err
		}
		g.CallSite = &cs
	}
	t := &traversal{e: e, o: &o, g: g, depth: map[int64]int{}}
	var level []int64
	for _, id := range tg.Funcs {
		if err := t.addNode(id, 0); err != nil {
			return nil, err
		}
		f, _ := e.fn(id)
		g.Start = append(g.Start, f.FID)
		level = append(level, id)
	}
	dynamicSeen := false
	for d := 1; d <= o.Depth && len(level) > 0; d++ {
		var next []int64
		for i, id := range level {
			if t.cut {
				g.Truncated.Unexpanded += len(level) - i
				break
			}
			edges, err := e.db.InEdges(id, graphdb.AllEdges)
			if err != nil {
				return nil, err
			}
			for j := range edges {
				ed := &edges[j]
				if !o.pass(ed) {
					continue
				}
				// the caller is a node of the next depth, unless it is known
				seen := false
				if _, ok := t.depth[ed.From]; ok {
					seen = true
				}
				if ok, err := t.emit(ed); err != nil {
					return nil, err
				} else if !ok {
					continue
				}
				if dynamic(ed.Kind) {
					dynamicSeen = true
				}
				if !seen {
					if err := t.addNode(ed.From, d); err != nil {
						return nil, err
					}
					next = append(next, ed.From)
				}
			}
		}
		level = next
	}
	if t.cut {
		g.Truncated.Unexpanded += len(level)
		g.Truncated.AtLeast = g.Truncated.Unexpanded > 0
		g.Hints = append(g.Hints, fmt.Sprintf("cut at %d edges: raise --limit, or ask for the callers of one of the nodes", o.Limit))
	} else {
		for _, id := range level {
			if has, err := e.db.HasIn(id); err != nil {
				return nil, err
			} else if has {
				g.Truncated.BeyondDepth++
			}
		}
		if g.Truncated.BeyondDepth > 0 {
			g.Hints = append(g.Hints, fmt.Sprintf("%d functions at depth %d have callers that are not shown: --depth %d goes further", g.Truncated.BeyondDepth, o.Depth, min(o.Depth+1, MaxDepth)))
		}
	}
	if dynamicSeen {
		g.Hints = append(g.Hints, "an edge of kind interface or value is one of \"candidates\" the call may reach: the caller may call this function, not certainly")
	}
	if tg.Builtin > 0 {
		g.Hints = append(g.Hints, "calls of builtins on the line were left out: --include-builtin keeps them")
	}
	return res, nil
}

// Callees answers what the function calls (or what the call a position names
// reaches). The static calls are followed. A dynamic call with more than
// ExpandCandidates candidates is where it stops: it is listed in the frontier,
// with its kind and number of candidates.
func (e *Engine) Callees(target string, o Options) (*CalleesResult, error) {
	clamped := o.normalize(DefaultLimit)
	tg, err := e.Resolve(target, o.IncludeBuiltin)
	if err != nil {
		return nil, err
	}
	g, _ := e.newGraph("callees", o, target, tg, "edges")
	res := &CalleesResult{Graph: *g, Frontier: []Frontier{}}
	g = &res.Graph
	if clamped {
		g.Warnings = append(g.Warnings, fmt.Sprintf("--depth is limited to %d", MaxDepth))
	}
	if tg.Ambiguous != nil {
		return res, e.ambiguousResult(g, tg)
	}
	t := &traversal{e: e, o: &o, g: g, depth: map[int64]int{}}
	var level []int64
	first := 1
	if tg.Call != nil {
		// the call is the start: its callee functions are depth 1
		cs, err := e.callSiteOut(tg.Call)
		if err != nil {
			return nil, err
		}
		g.CallSite = &cs
		if err := t.addNode(tg.Funcs[0], 0); err != nil {
			return nil, err
		}
		f, _ := e.fn(tg.Funcs[0])
		g.Start = append(g.Start, f.FID)
		edges, err := e.edgesOfCall(tg)
		if err != nil {
			return nil, err
		}
		for i := range edges {
			ed := &edges[i]
			if !o.pass(ed) {
				continue
			}
			if ok, err := t.emit(ed); err != nil {
				return nil, err
			} else if !ok {
				continue
			}
			if _, seen := t.depth[ed.To]; !seen {
				if err := t.addNode(ed.To, 1); err != nil {
					return nil, err
				}
				level = append(level, ed.To)
			}
		}
		if tg.Call.Kind == graphdb.KindBuiltin {
			// a builtin the person asked for: the call is the answer
			n := Node{ID: "builtin:" + builtinName(tg.Call.Callee), Depth: 1, Kind: "builtin"}
			g.Nodes = append(g.Nodes, n)
			g.Edges = append(g.Edges, EdgeOut{From: f.FID, To: n.ID, At: pos(tg.Call.File, tg.Call.Line, tg.Call.Col), Kind: graphdb.KindBuiltin})
		} else if len(edges) == 0 {
			g.Hints = append(g.Hints, "the call has no candidate: nothing in the graph says where it goes")
		}
		first = 2
	} else {
		for _, id := range tg.Funcs {
			if err := t.addNode(id, 0); err != nil {
				return nil, err
			}
			f, _ := e.fn(id)
			g.Start = append(g.Start, f.FID)
			level = append(level, id)
		}
	}
	filter := graphdb.EdgeFilter{MaxDynamic: ExpandCandidates}
	stopped := false
	for d := first; d <= o.Depth && len(level) > 0; d++ {
		var next []int64
		for i, id := range level {
			if t.cut {
				g.Truncated.Unexpanded += len(level) - i
				break
			}
			edges, err := e.db.OutEdges(id, filter)
			if err != nil {
				return nil, err
			}
			for j := range edges {
				ed := &edges[j]
				if !o.pass(ed) {
					continue
				}
				seen := false
				if _, ok := t.depth[ed.To]; ok {
					seen = true
				}
				if ok, err := t.emit(ed); err != nil {
					return nil, err
				} else if !ok {
					continue
				}
				if !seen {
					if err := t.addNode(ed.To, d); err != nil {
						return nil, err
					}
					next = append(next, ed.To)
				}
			}
			stop, err := e.db.UnexpandedCalls(id, ExpandCandidates)
			if err != nil {
				return nil, err
			}
			for k := range stop {
				stopped = true
				res.FrontierTotal++
				if len(res.Frontier) >= MaxFrontier {
					continue
				}
				c, err := e.frontier(&stop[k])
				if err != nil {
					return nil, err
				}
				res.Frontier = append(res.Frontier, c)
			}
			if err := e.builtins(t, id); err != nil {
				return nil, err
			}
		}
		level = next
	}
	if t.cut {
		g.Truncated.Unexpanded += len(level)
		g.Truncated.AtLeast = g.Truncated.Unexpanded > 0
		g.Hints = append(g.Hints, fmt.Sprintf("cut at %d edges: raise --limit, or ask for the callees of one of the nodes", o.Limit))
	} else {
		for _, id := range level {
			if has, err := e.db.HasOut(id); err != nil {
				return nil, err
			} else if has {
				g.Truncated.BeyondDepth++
			}
		}
		if g.Truncated.BeyondDepth > 0 {
			g.Hints = append(g.Hints, fmt.Sprintf("%d functions at depth %d call more than is shown: --depth %d goes further", g.Truncated.BeyondDepth, o.Depth, min(o.Depth+1, MaxDepth)))
		}
	}
	if stopped {
		g.Hints = append(g.Hints, fmt.Sprintf("stopped at %d dynamic calls (more than %d candidates, or none): the frontier lists them; callees <at> shows the candidates of one", res.FrontierTotal, ExpandCandidates))
	}
	if g.Omitted["builtin"] > 0 {
		g.Hints = append(g.Hints, "builtin calls (len, append...) are left out: --include-builtin keeps them")
	}
	return res, nil
}

func (e *Engine) frontier(c *graphdb.CallRow) (Frontier, error) {
	f, err := e.fn(c.Func)
	if err != nil {
		return Frontier{}, err
	}
	sites, err := e.siteIDs(c.ID)
	if err != nil {
		return Frontier{}, err
	}
	return Frontier{From: f.FID, At: pos(c.File, c.Line, c.Col), Kind: c.Kind, Candidates: c.NCand, Callee: c.Callee, Sites: sites}, nil
}

// builtins counts the builtin calls of an expanded function, or with
// --include-builtin adds them as edges.
func (e *Engine) builtins(t *traversal, id int64) error {
	if !t.o.IncludeBuiltin {
		n, err := e.db.BuiltinCount(id)
		if err != nil {
			return err
		}
		t.g.Omitted["builtin"] += n
		return nil
	}
	rows, err := e.db.BuiltinCalls(id)
	if err != nil {
		return err
	}
	f, err := e.fn(id)
	if err != nil {
		return err
	}
	for i := range rows {
		c := &rows[i]
		if t.nEdges >= t.o.Limit {
			t.cut = true
			t.g.Truncated.Count++
			continue
		}
		nid := "builtin:" + builtinName(c.Callee)
		if _, ok := t.builtinSeen(nid); !ok {
			t.g.Nodes = append(t.g.Nodes, Node{ID: nid, Depth: t.depth[id] + 1, Kind: "builtin"})
		}
		t.g.Edges = append(t.g.Edges, EdgeOut{From: f.FID, To: nid, At: pos(c.File, c.Line, c.Col), Kind: graphdb.KindBuiltin})
		t.nEdges++
	}
	return nil
}

func (t *traversal) builtinSeen(id string) (struct{}, bool) {
	for i := range t.g.Nodes {
		if t.g.Nodes[i].ID == id {
			return struct{}{}, true
		}
	}
	return struct{}{}, false
}

// builtinName is the name of a builtin as the language writes it ("builtin.len" is "len").
func builtinName(callee string) string { return strings.TrimPrefix(callee, "builtin.") }

// edgesOfCall are the edges of the call a target names (of all the calls at its place, without a function twice).
func (e *Engine) edgesOfCall(tg *Target) ([]graphdb.Edge, error) {
	edges, err := e.db.EdgesOfCallSite(tg.Call.ID)
	if err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	for _, ed := range edges {
		seen[ed.To] = true
	}
	for _, c := range tg.Same {
		more, err := e.db.EdgesOfCallSite(c.ID)
		if err != nil {
			return nil, err
		}
		for _, ed := range more {
			if !seen[ed.To] {
				seen[ed.To] = true
				edges = append(edges, ed)
			}
		}
	}
	return edges, nil
}
