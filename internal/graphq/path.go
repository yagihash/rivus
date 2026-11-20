package graphq

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/graphdb"
)

// adjacency reads the neighbours of the functions once each, by one filter.
type adjacency struct {
	e    *Engine
	f    graphdb.EdgeFilter
	pred EdgePred
	out  map[int64][]graphdb.Edge
	in   map[int64][]graphdb.Edge
	// hubs are the dynamic calls with more than HubCandidates candidates that the
	// search met (by their call site, once each), when it does not go through them.
	hubs map[int64]int
	// hubQuery is set when the calls that the filter leaves out are to be named.
	hubQuery bool
	visited  int
}

func (e *Engine) newAdj(f graphdb.EdgeFilter, pred EdgePred, hubs bool) *adjacency {
	return &adjacency{e: e, f: f, pred: pred, out: map[int64][]graphdb.Edge{}, in: map[int64][]graphdb.Edge{},
		hubs: map[int64]int{}, hubQuery: hubs}
}

// rank orders the edges between two functions: a static one, then a dynamic
// one with a modest number of candidates, then a hub.
func rank(e *graphdb.Edge) int {
	switch {
	case e.Static():
		return 0
	case e.NCand <= HubCandidates:
		return 1
	}
	return 2
}

// isHub: the call has more candidates than HubCandidates.
func isHub(e *graphdb.Edge) bool { return !e.Static() && e.NCand > HubCandidates }

// best keeps one edge of the ones between two functions: the one of the
// smallest rank (a static call, then a dynamic one of few candidates, then a
// hub), and of the same rank the first in the file.
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
			if rank(&ed) < rank(&out[j]) {
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
		if refs, err := a.e.db.HubOut(id, HubCandidates); err != nil {
			return nil, err
		} else {
			a.noteHubs(refs)
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
		if refs, err := a.e.db.HubIn(id, HubCandidates); err != nil {
			return nil, err
		} else {
			a.noteHubs(refs)
		}
	}
	return v, nil
}

func (a *adjacency) noteHubs(refs []graphdb.HubRef) {
	for _, r := range refs {
		a.hubs[r.ID] = r.NCand
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

// hubCost is what a call of many candidates costs in the search of the third
// stage, against 1 for a hop: a path is better by the fewer hubs it goes
// through, then by the fewer hops. It is more than any path's length.
const hubCost = 1 << 20

type pqItem struct {
	cost int
	id   int64
}

// pq is a heap of the functions to settle, the cheapest first.
type pq []pqItem

func (h *pq) push(it pqItem) {
	*h = append(*h, it)
	i := len(*h) - 1
	for i > 0 {
		p := (i - 1) / 2
		if (*h)[p].cost <= (*h)[i].cost {
			break
		}
		(*h)[p], (*h)[i] = (*h)[i], (*h)[p]
		i = p
	}
}

func (h *pq) pop() pqItem {
	old := *h
	top := old[0]
	n := len(old) - 1
	old[0] = old[n]
	*h = old[:n]
	i := 0
	for {
		l, r, m := 2*i+1, 2*i+2, i
		if l < n && (*h)[l].cost < (*h)[m].cost {
			m = l
		}
		if r < n && (*h)[r].cost < (*h)[m].cost {
			m = r
		}
		if m == i {
			break
		}
		(*h)[m], (*h)[i] = (*h)[i], (*h)[m]
		i = m
	}
	return top
}

// enumerateHub lists up to want paths, when the hubs are among the edges: the
// ones through the fewest hubs first, and among them the shortest (to
// PathSlack hops more than the best). The distance to a goal is the cost of
// the way, backward, settled as far as the longest path wanted.
func (a *adjacency) enumerateHub(starts, goals []int64, want int) (paths [][]graphdb.Edge, roots []int64, bestCost int, cut bool, err error) {
	w := func(ed *graphdb.Edge) int {
		if isHub(ed) {
			return hubCost + 1
		}
		return 1
	}
	isStart := map[int64]bool{}
	for _, s := range starts {
		isStart[s] = true
	}
	goal := map[int64]bool{}
	dist := map[int64]int{}
	var h pq
	for _, g := range goals {
		goal[g] = true
		dist[g] = 0
		h.push(pqItem{0, g})
	}
	best := -1
	for len(h) > 0 {
		it := h.pop()
		if it.cost > dist[it.id] {
			continue
		}
		if best >= 0 && it.cost > best+PathSlack {
			break
		}
		if best < 0 && isStart[it.id] {
			best = it.cost
		}
		if a.visited > MaxVisited {
			cut = true
			break
		}
		edges, e := a.prev(it.id)
		if e != nil {
			return nil, nil, 0, false, e
		}
		for i := range edges {
			ed := &edges[i]
			nc := it.cost + w(ed)
			if old, ok := dist[ed.From]; !ok || nc < old {
				dist[ed.From] = nc
				h.push(pqItem{nc, ed.From})
			}
		}
	}
	if best < 0 {
		return nil, nil, 0, cut, nil
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	steps := 0
	for c := best; c <= best+PathSlack && len(paths) < want; c++ {
		var stack []graphdb.Edge
		onPath := map[int64]bool{}
		var dfs func(n int64, used int, root int64) error
		dfs = func(n int64, used int, root int64) error {
			if len(paths) >= want || cut {
				return nil
			}
			steps++
			if steps > MaxSteps {
				cut = true
				return nil
			}
			if goal[n] {
				if used == c {
					paths = append(paths, append([]graphdb.Edge(nil), stack...))
					roots = append(roots, root)
				}
				return nil
			}
			edges, err := a.next(n)
			if err != nil {
				return err
			}
			for i := range edges {
				ed := &edges[i]
				d, ok := dist[ed.To]
				if !ok || onPath[ed.To] || used+w(ed)+d > c {
					continue
				}
				onPath[ed.To] = true
				stack = append(stack, *ed)
				if err := dfs(ed.To, used+w(ed), root); err != nil {
					return err
				}
				stack = stack[:len(stack)-1]
				onPath[ed.To] = false
			}
			return nil
		}
		for _, s := range starts {
			if d, ok := dist[s]; !ok || d > c {
				continue
			}
			onPath[s] = true
			if err := dfs(s, 0, s); err != nil {
				return nil, nil, 0, false, err
			}
			onPath[s] = false
		}
	}
	return paths, roots, best, cut, nil
}

// found is a path of edges, and the function it starts at.
type found struct {
	edges []graphdb.Edge
	root  int64
}

func (f found) end() int64 {
	if len(f.edges) == 0 {
		return f.root
	}
	return f.edges[len(f.edges)-1].To
}

func (f found) hubs() int {
	n := 0
	for i := range f.edges {
		if isHub(&f.edges[i]) {
			n++
		}
	}
	return n
}

// simple: no function twice.
func (f found) simple() bool {
	seen := map[int64]bool{f.root: true}
	for i := range f.edges {
		if seen[f.edges[i].To] {
			return false
		}
		seen[f.edges[i].To] = true
	}
	return true
}

// stageNames are the kinds of edges, in the order of the search.
var stageNames = []string{"static", "dynamic", "hub"}

func stageFilter(s int) graphdb.EdgeFilter {
	switch s {
	case 0:
		return graphdb.EdgeFilter{StaticOnly: true}
	case 1:
		return graphdb.EdgeFilter{MaxDynamic: HubCandidates}
	}
	return graphdb.AllEdges
}

type stageRes struct {
	found      bool
	paths      []found
	exhaustive bool
	cut        bool
	visited    int
	hubTotal   int
	// the range the paths cover: the length of the shortest path (the sum for
	// the stretches of the --via points), how many stretches, and (the hub
	// stage) the fewest hubs.
	shortHops, segs, hubs int
}

// reaching are the functions of ids from which a goal is reached.
func (a *adjacency) reaching(ids, goals []int64) ([]int64, error) {
	var live []int64
	for _, id := range ids {
		b, err := a.shortest([]int64{id}, goals)
		if err != nil {
			return nil, err
		}
		if b.found {
			live = append(live, id)
		}
	}
	return live, nil
}

// runStage searches with the edges of a stage. The points are the start, the
// --via functions in order, and the goal: the path is a way for each pair. A
// stretch begins where the paths of the one before ended (a --via function that
// has several IDs, as a method and its wrapper, is entered by one and left by
// the one that the stretch after it can use).
func (e *Engine) runStage(s int, points [][]int64, o *Options, want int) (stageRes, error) {
	a := e.newAdj(stageFilter(s), o.Edge, s == 1)
	pts := append([][]int64(nil), points...)
	narrowed := map[int]bool{}
	fail := func(exhaustive, cut bool) stageRes {
		return stageRes{exhaustive: exhaustive, cut: cut, visited: a.visited, hubTotal: len(a.hubs)}
	}
restart:
	for {
		var joined []found
		cut := false
		var short, hubs, segs int
		for i := 0; i+1 < len(pts); i++ {
			starts, goals := pts[i], pts[i+1]
			if i > 0 {
				seen := map[int64]bool{}
				starts = nil
				for _, x := range joined {
					if end := x.end(); !seen[end] {
						seen[end] = true
						starts = append(starts, end)
					}
				}
				sort.Slice(starts, func(p, q int) bool { return starts[p] < starts[q] })
			}
			b, err := a.shortest(append([]int64(nil), starts...), goals)
			if err != nil {
				return stageRes{}, err
			}
			if !b.found {
				if i > 0 && b.exhaustive && !narrowed[i] {
					// none of the ends of the paths so far goes on: another ID of the point may
					live, err := a.reaching(pts[i], goals)
					if err != nil {
						return stageRes{}, err
					}
					if len(live) > 0 {
						narrowed[i] = true
						pts[i] = live
						continue restart
					}
				}
				return fail(b.exhaustive, false), nil
			}
			var seg []found
			var c bool
			if s == 2 {
				ps, roots, best, c2, err := a.enumerateHub(append([]int64(nil), starts...), goals, want)
				if err != nil {
					return stageRes{}, err
				}
				c = c2
				for j := range ps {
					seg = append(seg, found{ps[j], roots[j]})
				}
				hubs += best / hubCost
				short += best % hubCost
			} else {
				ps, roots, c2, err := a.enumerate(append([]int64(nil), starts...), goals, b.length, want)
				if err != nil {
					return stageRes{}, err
				}
				c = c2
				for j := range ps {
					seg = append(seg, found{ps[j], roots[j][0]})
				}
				short += b.length
			}
			segs++
			cut = cut || c
			if len(seg) == 0 {
				// reached, but the listing was cut before one: say it is not exhaustive
				return fail(false, true), nil
			}
			if i == 0 {
				joined = seg
				continue
			}
			var next []found
			for _, x := range joined {
				for _, y := range seg {
					if y.root == x.end() {
						next = append(next, found{append(append([]graphdb.Edge(nil), x.edges...), y.edges...), x.root})
					}
				}
			}
			sort.SliceStable(next, func(p, q int) bool {
				if next[p].simple() != next[q].simple() {
					return next[p].simple()
				}
				if next[p].hubs() != next[q].hubs() {
					return next[p].hubs() < next[q].hubs()
				}
				return len(next[p].edges) < len(next[q].edges)
			})
			if len(next) > want {
				next = next[:want]
			}
			if len(next) == 0 {
				return fail(false, true), nil
			}
			joined = next
		}
		return stageRes{found: true, paths: joined, exhaustive: true, cut: cut, visited: a.visited, hubTotal: len(a.hubs), shortHops: short, segs: segs, hubs: hubs}, nil
	}
}

type searchRes struct {
	stageRes
	// stage is the one that found (an index of stageNames); 3 if none did.
	stage    int
	searched []string
}

// search looks with the static edges, then (if there is no path) with the
// dynamic ones that have at most HubCandidates candidates, then (if there is
// still none) with every edge. With o.Hubs it begins with the last.
func (e *Engine) search(points [][]int64, o *Options, want int) (*searchRes, error) {
	s := 0
	if o.Hubs {
		s = 2
	}
	var r stageRes
	for ; s <= 2; s++ {
		var err error
		if r, err = e.runStage(s, points, o, want); err != nil {
			return nil, err
		}
		// a stage whose listing was cut before a path is not "no path": the next one is not
		// searched in its place (it could give a path of a lesser kind while a better one exists)
		if r.found || r.cut {
			break
		}
	}
	return &searchRes{stageRes: r, stage: s, searched: append([]string(nil), stageNames[:min(s, 2)+1]...)}, nil
}

// Path answers the ways from a function (or a call) to another. The search is
// of three kinds of edges, one after the other: the static ones (what it finds
// is certain); if there is no path, the dynamic ones whose call has at most
// HubCandidates candidates (uncertain); if there is still none, every edge,
// the calls with more candidates (hubs) too. A path through a hub is the last
// resort: it is listed with the fewest hubs first. "searched" says how far the
// search went.
func (e *Engine) Path(from, to string, o Options) (*PathResult, error) {
	if o.Limit <= 0 {
		o.Limit = DefaultPaths
	}
	res := &PathResult{Common: e.common("path", o), Paths: []Path{}, Searched: []string{}}
	var src *Target
	var entries *graphdb.Entries
	var err error
	if o.FromEntry {
		if from != "" {
			return nil, fmt.Errorf("--from-entry and --from: name one start")
		}
		if entries, err = e.db.EntryFuncs(); err != nil {
			return nil, err
		}
		if entries.Source == "" {
			return nil, fmt.Errorf("--from-entry: the graph says no function where a program starts (no main or init, and no exported function of the code that was read)")
		}
		src = &Target{Funcs: entries.IDs, Desc: fmt.Sprintf("the %d entries (source: %s)", len(entries.IDs), entries.Source)}
		res.Entry = entryInfo(entries)
		from = "<entry>"
	} else if src, err = e.Resolve(from, o.IncludeBuiltin); err != nil {
		return nil, fmt.Errorf("--from: %w", err)
	}
	dst, err := e.Resolve(to, o.IncludeBuiltin)
	if err != nil {
		return nil, fmt.Errorf("--to: %w", err)
	}
	asked := o
	if err := e.prepare(&o, &res.Common); err != nil {
		return nil, err
	}
	if len(o.via) > 8 {
		return nil, fmt.Errorf("--via: at most 8 functions")
	}
	res.Query.From, res.Query.To = from, to
	res.Query.ResolvedFrom, res.Query.ResolvedTo = src.Desc, dst.Desc
	res.Truncated = Truncated{Limit: o.Limit, Unit: "paths"}
	res.Omitted["builtin"] += src.Builtin + dst.Builtin
	if src.isAmbiguous() || dst.isAmbiguous() {
		for _, t := range []*Target{src, dst} {
			if !t.isAmbiguous() {
				continue
			}
			c, err := e.choices(t)
			if err != nil {
				return nil, err
			}
			res.Ambiguous = append(res.Ambiguous, c.Ambiguous...)
			res.AmbiguousTotal += c.AmbiguousTotal
			res.Hints = append(res.Hints, ambiguousHints(t, c)...)
		}
		res.Exhaustive = true
		return res, nil
	}

	// A call as the start: the path begins in the function that holds it, goes through that call.
	var prefix []Step
	startsFor := func(filtered bool) ([]int64, error) {
		if src.Call == nil {
			return append([]int64(nil), src.Funcs...), nil
		}
		edges, err := e.edgesOfCall(src)
		if err != nil {
			return nil, err
		}
		var starts []int64
		for i := range edges {
			if !filtered || o.pass(&edges[i]) {
				starts = append(starts, edges[i].To)
			}
		}
		return starts, nil
	}
	starts, err := startsFor(true)
	if err != nil {
		return nil, err
	}
	if src.Call != nil {
		st, err := e.stepOfCall(src.Call, src.Funcs[0])
		if err != nil {
			return nil, err
		}
		prefix = []Step{st}
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

	points := [][]int64{starts}
	points = append(points, o.via...)
	points = append(points, goals)
	sr, err := e.search(points, &o, o.Limit+1)
	if err != nil {
		return nil, err
	}
	res.Searched = sr.searched
	res.Exhaustive = sr.exhaustive && !sr.cut
	if sr.found && sr.stage == 1 {
		res.HubSkippedTotal = sr.hubTotal
	}
	if !sr.found {
		res.Certainty = ""
		if !sr.exhaustive {
			res.Warnings = append(res.Warnings, fmt.Sprintf("the search was cut (%d functions visited, at most %d hops): \"no path\" is not proved", sr.visited, MaxPathHops))
		}
		what := "no path by the static calls, nor by the dynamic calls (with at most " + fmt.Sprint(HubCandidates) + " candidates, or more)"
		if o.filtered() {
			what += ", under the filters"
		}
		if res.Exhaustive {
			what += ": none in the graph file" + map[bool]string{true: " (under the filters)", false: ""}[o.filtered()]
		}
		res.Hints = append(res.Hints, what)
		if o.filtered() {
			wf, err := e.withoutFilters(&asked, src, dst, startsFor, res.Exhaustive)
			if err != nil {
				return nil, err
			}
			res.WithoutFilters = wf
			switch {
			case wf.Reachable && res.Exhaustive:
				res.Hints = append(res.Hints, fmt.Sprintf("without the filters there is a path (%s, %d hops): one of them cuts it; take one off", wf.Certainty, wf.Hops))
				if len(wf.Culprits) > 0 {
					res.Hints = append(res.Hints, "taking off only one of these lets a path through (the others kept): "+strings.Join(wf.Culprits, "; "))
				}
			case wf.Reachable:
				res.Hints = append(res.Hints, fmt.Sprintf("without the filters there is a path (%s, %d hops), but the search under the filters was cut by a bound (%d functions visited, at most %d hops): it cannot be said that the filters are the cause", wf.Certainty, wf.Hops, sr.visited, MaxPathHops))
			case wf.Exhaustive:
				res.Hints = append(res.Hints, "without the filters there is no path either")
			default:
				res.Hints = append(res.Hints, "without the filters the search was cut by a bound too: \"no path\" is not proved with or without them")
			}
		}
		if o.FromEntry {
			res.Hints = append(res.Hints, "no entry reaches it by calls: a function that is only passed as a value (a handler registered with a framework, a callback) is reached from the entry only through a call of a function value that lists it; \"no path\" does not say the code is dead")
		}
		res.Hints = append(res.Hints, "callees of the start, with --depth, shows where it goes (and the frontier, where it stops)")
		return res, nil
	}
	res.Reachable = true
	paths := sr.paths
	if len(paths) > o.Limit {
		paths = paths[:o.Limit]
		res.Truncated.Count = 1
		res.Truncated.AtLeast = true
	}
	for _, p := range paths {
		out, err := e.pathOut(p.root, p.edges, prefix, suffix)
		if err != nil {
			return nil, err
		}
		res.Paths = append(res.Paths, out)
	}
	res.Certainty = res.Paths[0].Certainty
	res.Listed = &Listed{Stage: stageNames[sr.stage], ShortestHops: sr.shortHops, MaxHops: sr.shortHops + PathSlack*sr.segs}
	if sr.stage == 2 {
		h := sr.hubs
		res.Listed.Hubs = &h
	}
	res.Beyond = "not_searched"
	switch res.Certainty {
	case "uncertain":
		res.Hints = append(res.Hints, "no path by the static calls alone: these go through an interface or a function value; \"candidates\" is the number of functions that call may reach")
		if res.HubSkippedTotal > 0 {
			res.Hints = append(res.Hints, fmt.Sprintf("%d dynamic calls with more than %d candidates (hubs) were met and not used: paths through them were not searched (\"searched\"); --hubs asks for them", res.HubSkippedTotal, HubCandidates))
		}
	case "hub":
		res.Hints = append(res.Hints,
			fmt.Sprintf("no path without a call of more than %d candidates (a hub): the paths listed go through the fewest of them, then the shortest; a step with \"hub\" is such a call, and \"candidates\" says how many functions it may reach", HubCandidates),
			"such a path may be real or not: to check one, ask from the implementation the code uses (--from the method, or the type it is made of), narrow the call with --only-impl <type>, or cut a hub: --avoid-call <file:line:col> cuts one call site (an \"at\" of the path), --avoid <function> a whole function")
	}
	if o.filtered() {
		res.Hints = append(res.Hints, "the paths are those that the filters above (\"filters\") allow")
	}
	if o.FromEntry {
		res.Hints = append(res.Hints, "the first step of each path is the entry it starts from (\"entry\" says what an entry is)")
	}
	if res.Truncated.Count > 0 {
		res.Hints = append(res.Hints, "more paths exist: raise --limit")
	}
	if sr.cut {
		res.Warnings = append(res.Warnings, "the listing of the paths was cut by a bound: others may exist")
	}
	return res, nil
}

// withoutFilters asks the question again without the filters (--via too), to
// say whether they are what stops the path. With more than one filter it asks
// again with each one taken off alone, and names the ones that let a path
// through. o is what was asked, before the filters were made into a predicate.
func (e *Engine) withoutFilters(o *Options, src, dst *Target, startsFor func(bool) ([]int64, error), whole bool) (*WithoutFilters, error) {
	starts, err := startsFor(false)
	if err != nil {
		return nil, err
	}
	plain := Options{Limit: 1, Hubs: o.Hubs}
	sr, err := e.search([][]int64{starts, dst.Funcs}, &plain, 1)
	if err != nil {
		return nil, err
	}
	wf := &WithoutFilters{Reachable: sr.found, Searched: sr.searched, Exhaustive: sr.exhaustive && !sr.cut}
	if sr.found {
		wf.Certainty = []string{"certain", "uncertain", "hub"}[sr.stage]
		wf.Hops = len(sr.paths[0].edges)
	}
	if !sr.found || !whole || o.nFilters() < 2 {
		return wf, nil
	}
	// each filter taken off alone, the others kept
	type drop struct {
		name string
		make func(Options) Options
	}
	var drops []drop
	cut := func(l []string, i int) []string { return append(append([]string(nil), l[:i]...), l[i+1:]...) }
	for i := range o.Avoid {
		i := i
		drops = append(drops, drop{"--avoid " + o.Avoid[i], func(x Options) Options { x.Avoid = cut(x.Avoid, i); return x }})
	}
	for i := range o.AvoidCall {
		i := i
		drops = append(drops, drop{"--avoid-call " + o.AvoidCall[i], func(x Options) Options { x.AvoidCall = cut(x.AvoidCall, i); return x }})
	}
	for i := range o.Via {
		i := i
		drops = append(drops, drop{"--via " + o.Via[i], func(x Options) Options { x.Via = cut(x.Via, i); return x }})
	}
	for i := range o.OnlyImpl {
		i := i
		drops = append(drops, drop{"--only-impl " + o.OnlyImpl[i], func(x Options) Options { x.OnlyImpl = cut(x.OnlyImpl, i); return x }})
	}
	for _, d := range drops {
		x := d.make(*o)
		var c Common
		if err := e.prepare(&x, &c); err != nil {
			return nil, err
		}
		st := starts
		if src.Call != nil {
			if st, err = startsWith(e, src, &x); err != nil {
				return nil, err
			}
		}
		pts := append(append([][]int64{st}, x.via...), dst.Funcs)
		r, err := e.search(pts, &x, 1)
		if err != nil {
			return nil, err
		}
		if r.found {
			wf.Culprits = append(wf.Culprits, d.name)
		}
	}
	return wf, nil
}

// startsWith are the functions a call as the start reaches, under the filters of o.
func startsWith(e *Engine, src *Target, o *Options) ([]int64, error) {
	edges, err := e.edgesOfCall(src)
	if err != nil {
		return nil, err
	}
	var starts []int64
	for i := range edges {
		if o.pass(&edges[i]) {
			starts = append(starts, edges[i].To)
		}
	}
	return starts, nil
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
	st := Step{Func: f.FID, At: pos(c.File, c.Line, c.Col), Kind: c.Kind, Sites: sites, NoBody: !f.HasBody}
	if f.Symbol != f.FID {
		st.Symbol = f.Symbol
	}
	if dynamic(c.Kind) {
		st.Candidates = c.NCand
		st.Hub = c.NCand > HubCandidates
	}
	return st, nil
}

func (e *Engine) pathOut(start int64, edges []graphdb.Edge, prefix []Step, suffix *Step) (Path, error) {
	p := Path{Certainty: "certain"}
	p.Steps = append(p.Steps, prefix...)
	cur := start
	// raise the certainty of the path: certain, uncertain (a dynamic call), hub (a call of many candidates)
	raise := func(kind string, hub bool) {
		if hub {
			p.Certainty = "hub"
		} else if dynamic(kind) && p.Certainty == "certain" {
			p.Certainty = "uncertain"
		}
	}
	add := func(id int64, ed *graphdb.Edge) error {
		f, err := e.fn(id)
		if err != nil {
			return err
		}
		st := Step{Func: f.FID, NoBody: !f.HasBody}
		if f.Symbol != f.FID {
			st.Symbol = f.Symbol
		}
		if ed != nil {
			st.At = pos(ed.File, ed.Line, ed.Col)
			st.Kind = ed.Kind
			if dynamic(ed.Kind) {
				st.Candidates = ed.NCand
			}
			if isHub(ed) {
				st.Hub = true
			}
			raise(ed.Kind, st.Hub)
			if st.Sites, err = e.siteIDs(ed.CallSite); err != nil {
				return err
			}
		} else if f.File != "" && f.Line > 0 {
			// the end of the path: where the function is declared
			st.DefinedAt = pos(f.File, f.Line, f.Col)
		}
		p.Steps = append(p.Steps, st)
		return nil
	}
	if len(prefix) > 0 {
		raise(prefix[0].Kind, prefix[0].Hub)
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
		raise(suffix.Kind, suffix.Hub)
	} else if err := add(cur, nil); err != nil {
		return p, err
	}
	p.Hops = len(edges)
	return p, nil
}
