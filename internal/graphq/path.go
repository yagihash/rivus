package graphq

import (
	"fmt"
	"sort"

	"github.com/yagihash/rivus/internal/graphdb"
)

// adjacency reads the neighbours of the functions once each, by one filter.
type adjacency struct {
	e      *Engine
	f      graphdb.EdgeFilter
	pred   EdgePred
	out    map[int64][]graphdb.Edge
	in     map[int64][]graphdb.Edge
	hubs   map[int64]graphdb.CallRow
	hubTot map[int64]bool
	// hubQuery is set when the calls that the filter leaves out are to be named.
	hubQuery bool
	visited  int
}

func (e *Engine) newAdj(f graphdb.EdgeFilter, pred EdgePred, hubs bool) *adjacency {
	return &adjacency{e: e, f: f, pred: pred, out: map[int64][]graphdb.Edge{}, in: map[int64][]graphdb.Edge{},
		hubs: map[int64]graphdb.CallRow{}, hubTot: map[int64]bool{}, hubQuery: hubs}
}

// best keeps one edge of the ones between two functions: a static one first,
// then the first in the file.
func (a *adjacency) best(edges []graphdb.Edge, key func(*graphdb.Edge) int64) []graphdb.Edge {
	idx := map[int64]int{}
	var out []graphdb.Edge
	for i := range edges {
		ed := edges[i]
		if a.pred != nil && !a.pred(&ed) {
			continue
		}
		k := key(&ed)
		if j, ok := idx[k]; ok {
			if ed.Static() && !out[j].Static() {
				out[j] = ed
			}
			continue
		}
		idx[k] = len(out)
		out = append(out, ed)
	}
	return out
}

func (a *adjacency) next(id int64) ([]graphdb.Edge, error) {
	if v, ok := a.out[id]; ok {
		return v, nil
	}
	a.visited++
	edges, err := a.e.db.OutEdges(id, a.f)
	if err != nil {
		return nil, err
	}
	v := a.best(edges, func(e *graphdb.Edge) int64 { return e.To })
	a.out[id] = v
	if a.hubQuery {
		if n, rows, err := a.e.db.HubOut(id, HubCandidates, 3); err != nil {
			return nil, err
		} else if n > 0 {
			a.noteHubs(n, rows)
		}
	}
	return v, nil
}

func (a *adjacency) prev(id int64) ([]graphdb.Edge, error) {
	if v, ok := a.in[id]; ok {
		return v, nil
	}
	a.visited++
	edges, err := a.e.db.InEdges(id, a.f)
	if err != nil {
		return nil, err
	}
	v := a.best(edges, func(e *graphdb.Edge) int64 { return e.From })
	a.in[id] = v
	if a.hubQuery {
		if n, rows, err := a.e.db.HubIn(id, HubCandidates, 3); err != nil {
			return nil, err
		} else if n > 0 {
			a.noteHubs(n, rows)
		}
	}
	return v, nil
}

func (a *adjacency) noteHubs(n int, rows []graphdb.CallRow) {
	for _, r := range rows {
		if _, ok := a.hubs[r.ID]; !ok {
			a.hubs[r.ID] = r
		}
	}
}

// bound is the result of the search for the length of the shortest path.
type bound struct {
	found      bool
	length     int
	exhaustive bool
}

// shortest finds the length of the shortest path from a start to a goal, by a
// search from both ends (a layer of the smaller side at a time).
func (a *adjacency) shortest(starts, goals []int64) (bound, error) {
	df := map[int64]int{}
	db := map[int64]int{}
	var ff, fb []int64
	for _, s := range starts {
		df[s] = 0
		ff = append(ff, s)
	}
	for _, g := range goals {
		if _, ok := df[g]; ok {
			return bound{found: true, length: 0, exhaustive: true}, nil
		}
		db[g] = 0
		fb = append(fb, g)
	}
	rf, rb := 0, 0
	for len(ff) > 0 && len(fb) > 0 {
		if rf+rb >= MaxPathHops || a.visited > MaxVisited {
			return bound{}, nil
		}
		forward := len(ff) <= len(fb)
		var layer []int64
		best := -1
		if forward {
			rf++
			for _, id := range ff {
				edges, err := a.next(id)
				if err != nil {
					return bound{}, err
				}
				for _, ed := range edges {
					if _, ok := df[ed.To]; ok {
						continue
					}
					df[ed.To] = rf
					layer = append(layer, ed.To)
					if d, ok := db[ed.To]; ok && (best < 0 || rf+d < best) {
						best = rf + d
					}
				}
			}
			ff = layer
		} else {
			rb++
			for _, id := range fb {
				edges, err := a.prev(id)
				if err != nil {
					return bound{}, err
				}
				for _, ed := range edges {
					if _, ok := db[ed.From]; ok {
						continue
					}
					db[ed.From] = rb
					layer = append(layer, ed.From)
					if d, ok := df[ed.From]; ok && (best < 0 || rb+d < best) {
						best = rb + d
					}
				}
			}
			fb = layer
		}
		if best >= 0 {
			return bound{found: true, length: best, exhaustive: true}, nil
		}
	}
	// a side has nothing more to reach: there is no path (unless a bound cut the search, which returned above)
	return bound{exhaustive: true}, nil
}

// enumerate lists up to want paths of the functions, shortest first, none the
// same list of functions as another: those of length short, then of a hop
// more, up to PathSlack more. cut is set when a bound stopped the listing.
func (a *adjacency) enumerate(starts, goals []int64, short, want int) (paths [][]graphdb.Edge, startOf [][]int64, cut bool, err error) {
	goal := map[int64]bool{}
	for _, g := range goals {
		goal[g] = true
	}
	// the distance to a goal, backward, up to the longest length wanted
	maxLen := short + PathSlack
	dist := map[int64]int{}
	layer := append([]int64(nil), goals...)
	for _, g := range goals {
		dist[g] = 0
	}
	for r := 1; r <= maxLen && len(layer) > 0; r++ {
		var nl []int64
		for _, id := range layer {
			edges, e := a.prev(id)
			if e != nil {
				return nil, nil, false, e
			}
			for _, ed := range edges {
				if _, ok := dist[ed.From]; !ok {
					dist[ed.From] = r
					nl = append(nl, ed.From)
				}
			}
			if a.visited > MaxVisited {
				cut = true
				break
			}
		}
		layer = nl
		if cut {
			break
		}
	}
	steps := 0
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	for L := short; L <= maxLen && len(paths) < want; L++ {
		var stack []graphdb.Edge
		onPath := map[int64]bool{}
		var dfs func(n int64, depth int, root int64) error
		dfs = func(n int64, depth int, root int64) error {
			if len(paths) >= want || cut {
				return nil
			}
			steps++
			if steps > MaxSteps {
				cut = true
				return nil
			}
			if goal[n] {
				if depth == L {
					paths = append(paths, append([]graphdb.Edge(nil), stack...))
					startOf = append(startOf, []int64{root})
				}
				return nil
			}
			if depth >= L {
				return nil
			}
			edges, err := a.next(n)
			if err != nil {
				return err
			}
			for _, ed := range edges {
				d, ok := dist[ed.To]
				if !ok || depth+1+d > L || onPath[ed.To] {
					continue
				}
				onPath[ed.To] = true
				stack = append(stack, ed)
				if err := dfs(ed.To, depth+1, root); err != nil {
					return err
				}
				stack = stack[:len(stack)-1]
				onPath[ed.To] = false
			}
			return nil
		}
		for _, s := range starts {
			if _, ok := dist[s]; !ok {
				continue
			}
			onPath[s] = true
			if err := dfs(s, 0, s); err != nil {
				return nil, nil, false, err
			}
			onPath[s] = false
		}
	}
	return paths, startOf, cut, nil
}

// Path answers the ways from a function (or a call) to another. The search
// is first of the static edges alone: what it finds is certain. If there is
// none, it looks again with the dynamic edges (an interface, a function value)
// whose call has at most HubCandidates candidates; the paths are then uncertain.
func (e *Engine) Path(from, to string, o Options) (*PathResult, error) {
	if o.Limit <= 0 {
		o.Limit = DefaultPaths
	}
	src, err := e.Resolve(from, o.IncludeBuiltin)
	if err != nil {
		return nil, fmt.Errorf("--from: %w", err)
	}
	dst, err := e.Resolve(to, o.IncludeBuiltin)
	if err != nil {
		return nil, fmt.Errorf("--to: %w", err)
	}
	res := &PathResult{Common: e.common("path", o), Paths: []Path{}, HubSkipped: []HubSite{}}
	res.Query.From, res.Query.To = from, to
	res.Query.ResolvedFrom, res.Query.ResolvedTo = src.Desc, dst.Desc
	res.Truncated = Truncated{Limit: o.Limit, Unit: "paths"}
	res.Omitted["builtin"] += src.Builtin + dst.Builtin
	if src.Ambiguous != nil || dst.Ambiguous != nil {
		for _, t := range []*Target{src, dst} {
			amb, err := e.ambiguous(t.Ambiguous)
			if err != nil {
				return nil, err
			}
			res.Ambiguous = append(res.Ambiguous, amb...)
		}
		res.Exhaustive = true
		res.Hints = append(res.Hints, "a target names several calls: ask again with one of the \"at\" values (file:line:col), or site:<id>")
		return res, nil
	}

	// A call as the start: the path begins in the function that holds it, goes through that call.
	var starts []int64
	var prefix []Step
	if src.Call != nil {
		edges, err := e.edgesOfCall(src)
		if err != nil {
			return nil, err
		}
		for i := range edges {
			if o.pass(&edges[i]) {
				starts = append(starts, edges[i].To)
			}
		}
		st, err := e.stepOfCall(src.Call, src.Funcs[0])
		if err != nil {
			return nil, err
		}
		prefix = []Step{st}
	} else {
		starts = src.Funcs
	}
	// A call as the goal: the path ends in the function that holds it, and the last step is the call.
	goals := dst.Funcs
	var suffix *Step
	if dst.Call != nil {
		st, err := e.stepOfCall(dst.Call, dst.Funcs[0])
		if err != nil {
			return nil, err
		}
		suffix = &st
	}

	run := func(f graphdb.EdgeFilter, hubs bool) (*adjacency, [][]graphdb.Edge, [][]int64, bound, bool, error) {
		a := e.newAdj(f, o.Edge, hubs)
		b, err := a.shortest(append([]int64(nil), starts...), goals)
		if err != nil || !b.found {
			return a, nil, nil, b, false, err
		}
		paths, roots, cut, err := a.enumerate(append([]int64(nil), starts...), goals, b.length, o.Limit+1)
		return a, paths, roots, b, cut, err
	}

	a, paths, roots, b, cut, err := run(graphdb.EdgeFilter{StaticOnly: true}, false)
	if err != nil {
		return nil, err
	}
	res.Certainty = "certain"
	if !b.found {
		// again, with the dynamic edges that have a modest number of candidates
		a, paths, roots, b, cut, err = run(graphdb.EdgeFilter{MaxDynamic: HubCandidates}, true)
		if err != nil {
			return nil, err
		}
		res.Certainty = "uncertain"
		res.HubSkippedTotal = len(a.hubs)
		var hs []graphdb.CallRow
		for _, h := range a.hubs {
			hs = append(hs, h)
		}
		sort.Slice(hs, func(i, j int) bool {
			if hs[i].NCand != hs[j].NCand {
				return hs[i].NCand > hs[j].NCand
			}
			return hs[i].ID < hs[j].ID
		})
		for i := range hs {
			if i >= MaxHubs {
				break
			}
			f, err := e.fn(hs[i].Func)
			if err != nil {
				return nil, err
			}
			res.HubSkipped = append(res.HubSkipped, HubSite{In: f.FID, At: pos(hs[i].File, hs[i].Line, hs[i].Col), Kind: hs[i].Kind, Candidates: hs[i].NCand})
		}
	}
	res.Exhaustive = b.exhaustive && !cut
	if !b.found {
		res.Certainty = ""
		if !b.exhaustive {
			res.Warnings = append(res.Warnings, fmt.Sprintf("the search was cut (%d functions visited, at most %d hops): \"no path\" is not proved", a.visited, MaxPathHops))
		}
		res.Hints = append(res.Hints, "no path by the static calls, nor by the dynamic calls with at most "+fmt.Sprint(HubCandidates)+" candidates")
		if res.HubSkippedTotal > 0 {
			res.Hints = append(res.Hints, fmt.Sprintf("%d dynamic calls with more than %d candidates were not used (hub_skipped): a path through one may exist; look at which implementation the code uses, and ask from it", res.HubSkippedTotal, HubCandidates))
		}
		res.Hints = append(res.Hints, "callees of the start, with --depth, shows where it goes (and the frontier, where it stops)")
		return res, nil
	}
	res.Reachable = true
	if len(paths) > o.Limit {
		paths = paths[:o.Limit]
		res.Truncated.Count = 1
		res.Truncated.AtLeast = true
	}
	for i, p := range paths {
		out, err := e.pathOut(roots[i][0], p, prefix, suffix, res.Certainty == "uncertain")
		if err != nil {
			return nil, err
		}
		res.Paths = append(res.Paths, out)
	}
	if res.Certainty == "uncertain" {
		res.Hints = append(res.Hints, "no path by the static calls alone: these go through an interface or a function value; \"candidates\" is the number of functions that call may reach")
		if res.HubSkippedTotal > 0 {
			res.Hints = append(res.Hints, fmt.Sprintf("%d dynamic calls with more than %d candidates were not used (hub_skipped): other paths may go through them", res.HubSkippedTotal, HubCandidates))
		}
	}
	if res.Truncated.Count > 0 {
		res.Hints = append(res.Hints, "more paths exist: raise --limit")
	}
	if cut {
		res.Warnings = append(res.Warnings, "the listing of the paths was cut by a bound: others may exist")
	}
	return res, nil
}

func (e *Engine) stepOfCall(c *graphdb.CallRow, holder int64) (Step, error) {
	f, err := e.fn(holder)
	if err != nil {
		return Step{}, err
	}
	sites, err := e.siteIDs(c.ID)
	if err != nil {
		return Step{}, err
	}
	st := Step{Func: f.FID, At: pos(c.File, c.Line, c.Col), Kind: c.Kind, Sites: sites}
	if f.Symbol != f.FID {
		st.Symbol = f.Symbol
	}
	if dynamic(c.Kind) {
		st.Candidates = c.NCand
	}
	return st, nil
}

func (e *Engine) pathOut(start int64, edges []graphdb.Edge, prefix []Step, suffix *Step, uncertain bool) (Path, error) {
	p := Path{Certainty: "certain"}
	p.Steps = append(p.Steps, prefix...)
	cur := start
	add := func(id int64, ed *graphdb.Edge) error {
		f, err := e.fn(id)
		if err != nil {
			return err
		}
		st := Step{Func: f.FID}
		if f.Symbol != f.FID {
			st.Symbol = f.Symbol
		}
		if ed != nil {
			st.At = pos(ed.File, ed.Line, ed.Col)
			st.Kind = ed.Kind
			if dynamic(ed.Kind) {
				st.Candidates = ed.NCand
				p.Certainty = "uncertain"
			}
			if st.Sites, err = e.siteIDs(ed.CallSite); err != nil {
				return err
			}
		}
		p.Steps = append(p.Steps, st)
		return nil
	}
	if len(prefix) > 0 && prefix[0].Kind != "" && dynamic(prefix[0].Kind) {
		p.Certainty = "uncertain"
	}
	for i := range edges {
		if err := add(cur, &edges[i]); err != nil {
			return p, err
		}
		cur = edges[i].To
	}
	// the last function: when a call is the goal, it is the call
	if suffix != nil {
		p.Steps = append(p.Steps, *suffix)
		if dynamic(suffix.Kind) {
			p.Certainty = "uncertain"
		}
	} else if err := add(cur, nil); err != nil {
		return p, err
	}
	p.Hops = len(edges)
	return p, nil
}
