package graphq

import (
	"fmt"
	"strings"

	"github.com/yagihash/rivus/internal/graphdb"
)

// state is a function reached, and which of the --via functions the way to it
// went through (a bit each; with none given, the mask is always 0, which is full).
type state struct {
	id   int64
	mask uint
}

// traversal is the state of a breadth-first search over callers or callees.
type traversal struct {
	e      *Engine
	o      *Options
	g      *Graph
	depth  map[int64]int
	nEdges int
	// cut is true once the limit of edges is reached.
	cut bool
	// the --via functions: a node is shown only when the way to it went through all of them
	via     []map[int64]bool
	full    uint
	seen    map[state]bool
	emitted map[[3]int64]bool
	// work counts the states expanded that are not shown (their way has not yet been through the --via
	// functions): it is bounded by MaxVisited.
	work    int
	workCut bool
}

func newTraversal(e *Engine, o *Options, g *Graph) (*traversal, error) {
	t := &traversal{e: e, o: o, g: g, depth: map[int64]int{}, seen: map[state]bool{}, emitted: map[[3]int64]bool{}}
	if len(o.via) > 8 {
		return nil, fmt.Errorf("--via: at most 8 functions")
	}
	for _, ids := range o.via {
		m := map[int64]bool{}
		for _, id := range ids {
			m[id] = true
		}
		t.via = append(t.via, m)
	}
	t.full = uint(1)<<len(t.via) - 1
	return t, nil
}

func (t *traversal) bit(id int64) uint {
	var b uint
	for i, m := range t.via {
		if m[id] {
			b |= 1 << i
		}
	}
	return b
}

// reach is the state of going to a function from a state; shown says that the
// way to it went through every --via function, so that it is part of the answer.
func (t *traversal) reach(from state, to int64) (s state, shown bool) {
	s = state{to, from.mask | t.bit(to)}
	return s, s.mask == t.full
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
// then counted as cut). An edge that was added (by another way to its end, with --via) is not added twice.
func (t *traversal) emit(ed *graphdb.Edge) (bool, error) {
	key := [3]int64{ed.CallSite, ed.From, ed.To}
	if len(t.via) > 0 && t.emitted[key] {
		return true, nil
	}
	if t.nEdges >= t.o.Limit {
		t.cut = true
		t.g.Truncated.Count++
		return false, nil
	}
	out, err := t.e.edgeOut(ed)
	if err != nil {
		return false, err
	}
	t.emitted[key] = true
	t.g.Edges = append(t.g.Edges, out)
	t.nEdges++
	return true, nil
}

// work counts a state that is expanded without being shown, and reports that the bound is reached.
func (t *traversal) spend(s state) bool {
	if s.mask == t.full {
		return false
	}
	t.work++
	if t.work > MaxVisited {
		t.workCut = true
		return true
	}
	return false
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
	c, err := e.choices(tg)
	if err != nil {
		return err
	}
	g.Choices = c
	g.Hints = append(g.Hints, ambiguousHints(tg, c)...)
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
	if err := e.prepare(&o, &g.Common); err != nil {
		return nil, err
	}
	if clamped {
		g.Warnings = append(g.Warnings, fmt.Sprintf("--depth is limited to %d", MaxDepth))
	}
	if tg.isAmbiguous() {
		return res, e.ambiguousResult(g, tg)
	}
	if tg.Call != nil {
		cs, err := e.callSiteOut(tg.Call)
		if err != nil {
			return nil, err
		}
		g.CallSite = &cs
	}
	t, err := newTraversal(e, &o, g)
	if err != nil {
		return nil, err
	}
	var level []state
	for _, id := range tg.Funcs {
		s := state{id, t.bit(id)}
		t.seen[s] = true
		if err := t.addNode(id, 0); err != nil {
			return nil, err
		}
		f, _ := e.fn(id)
		g.Start = append(g.Start, f.FID)
		level = append(level, s)
	}
	dynamicSeen := false
	for d := 1; d <= o.Depth && len(level) > 0; d++ {
		var next []state
		for i, s := range level {
			if t.cut || t.workCut {
				g.Truncated.Unexpanded += len(level) - i
				break
			}
			if t.spend(s) {
				break
			}
			edges, err := e.db.InEdges(s.id, graphdb.AllEdges)
			if err != nil {
				return nil, err
			}
			for j := range edges {
				ed := &edges[j]
				if !o.pass(ed) {
					continue
				}
				ns, shown := t.reach(s, ed.From)
				if shown {
					if ok, err := t.emit(ed); err != nil {
						return nil, err
					} else if !ok {
						continue
					}
					if dynamic(ed.Kind) {
						dynamicSeen = true
					}
				}
				if !t.seen[ns] {
					t.seen[ns] = true
					if shown {
						if err := t.addNode(ed.From, d); err != nil {
							return nil, err
						}
					}
					next = append(next, ns)
				}
			}
		}
		level = next
	}
	if t.workCut {
		g.Warnings = append(g.Warnings, fmt.Sprintf("--via: the search was cut after %d functions that are not on the way: the answer may be short", MaxVisited))
	}
	if t.cut {
		g.Truncated.Unexpanded += len(level)
		g.Truncated.AtLeast = g.Truncated.Unexpanded > 0
		g.Hints = append(g.Hints, fmt.Sprintf("cut at %d edges: raise --limit, or ask for the callers of one of the nodes", o.Limit))
	} else {
		for _, s := range level {
			if s.mask != t.full {
				continue
			}
			if has, err := e.db.HasIn(s.id); err != nil {
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
	if o.filtered() {
		g.Hints = append(g.Hints, "the answer is narrowed by the filters above (\"filters\"): without them it has more")
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
	if err := e.prepare(&o, &g.Common); err != nil {
		return nil, err
	}
	if clamped {
		g.Warnings = append(g.Warnings, fmt.Sprintf("--depth is limited to %d", MaxDepth))
	}
	if tg.isAmbiguous() {
		return res, e.ambiguousResult(g, tg)
	}
	t, err := newTraversal(e, &o, g)
	if err != nil {
		return nil, err
	}
	var level []state
	first := 1
	if tg.Call != nil {
		// the call is the start: its callee functions are depth 1
		cs, err := e.callSiteOut(tg.Call)
		if err != nil {
			return nil, err
		}
		g.CallSite = &cs
		hs := state{tg.Funcs[0], t.bit(tg.Funcs[0])}
		t.seen[hs] = true
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
			ns, shown := t.reach(hs, ed.To)
			if shown {
				if ok, err := t.emit(ed); err != nil {
					return nil, err
				} else if !ok {
					continue
				}
			}
			if !t.seen[ns] {
				t.seen[ns] = true
				if shown {
					if err := t.addNode(ed.To, 1); err != nil {
						return nil, err
					}
				}
				level = append(level, ns)
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
			s := state{id, t.bit(id)}
			t.seen[s] = true
			if err := t.addNode(id, 0); err != nil {
				return nil, err
			}
			f, _ := e.fn(id)
			g.Start = append(g.Start, f.FID)
			level = append(level, s)
		}
	}
	filter := graphdb.EdgeFilter{MaxDynamic: ExpandCandidates}
	stopped := false
	for d := first; d <= o.Depth && len(level) > 0; d++ {
		var next []state
		for i, s := range level {
			if t.cut || t.workCut {
				g.Truncated.Unexpanded += len(level) - i
				break
			}
			if t.spend(s) {
				break
			}
			id := s.id
			edges, err := e.db.OutEdges(id, filter)
			if err != nil {
				return nil, err
			}
			for j := range edges {
				ed := &edges[j]
				if !o.pass(ed) {
					continue
				}
				ns, shown := t.reach(s, ed.To)
				if shown {
					if ok, err := t.emit(ed); err != nil {
						return nil, err
					} else if !ok {
						continue
					}
				}
				if !t.seen[ns] {
					t.seen[ns] = true
					if shown {
						if err := t.addNode(ed.To, d); err != nil {
							return nil, err
						}
					}
					next = append(next, ns)
				}
			}
			if s.mask != t.full {
				continue
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
	if t.workCut {
		g.Warnings = append(g.Warnings, fmt.Sprintf("--via: the search was cut after %d functions that are not on the way: the answer may be short", MaxVisited))
	}
	if t.cut {
		g.Truncated.Unexpanded += len(level)
		g.Truncated.AtLeast = g.Truncated.Unexpanded > 0
		g.Hints = append(g.Hints, fmt.Sprintf("cut at %d edges: raise --limit, or ask for the callees of one of the nodes", o.Limit))
	} else {
		for _, s := range level {
			if s.mask != t.full {
				continue
			}
			if has, err := e.db.HasOut(s.id); err != nil {
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
	if len(o.via) > 0 {
		g.Hints = append(g.Hints, "--via: the search does not go through a dynamic call with more than "+fmt.Sprint(ExpandCandidates)+" candidates, so a way through one is not shown (graph path --via asks for it)")
	}
	if o.filtered() {
		g.Hints = append(g.Hints, "the answer is narrowed by the filters above (\"filters\"): without them it has more")
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
