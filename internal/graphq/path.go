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
// one with a modest number of candidates, then a hub, then a reference (which
// is not a call).
func rank(e *graphdb.Edge) int {
	switch {
	case e.Static():
		return 0
	case isRef(e):
		return 3
	case e.NCand <= HubCandidates:
		return 1
	}
	return 2
}

// isRef: the edge is a function named as a value, not a call.
func isRef(e *graphdb.Edge) bool { return e.Kind == graphdb.KindRef }

// isHub: the call has more candidates than HubCandidates.
func isHub(e *graphdb.Edge) bool { return !e.Static() && !isRef(e) && e.NCand > HubCandidates }

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

// refCost is what a reference costs in the search of the fourth stage: a path is
// better by the fewer references it goes through, then by the fewer hubs, then
// by the fewer hops. It is more than any number of hubs.
const refCost = 1 << 40

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

// enumerateHub lists up to want paths, when the hubs (and, in the fourth stage,
// the references) are among the edges: the ones through the fewest references,
// then the fewest hubs first, and among them the shortest (to
// PathSlack hops more than the best). The distance to a goal is the cost of
// the way, backward, settled as far as the longest path wanted.
func (a *adjacency) enumerateHub(starts, goals []int64, want int) (paths [][]graphdb.Edge, roots []int64, bestCost int, cut bool, err error) {
	w := func(ed *graphdb.Edge) int {
		if isRef(ed) {
			return refCost + 1
		}
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

func (f found) refs() int {
	n := 0
	for i := range f.edges {
		if isRef(&f.edges[i]) {
			n++
		}
	}
	return n
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
var stageNames = []string{"static", "dynamic", "hub", "ref"}

func stageFilter(s int) graphdb.EdgeFilter {
	switch s {
	case 0:
		return graphdb.EdgeFilter{StaticOnly: true}
	case 1:
		return graphdb.EdgeFilter{MaxDynamic: HubCandidates}
	case 2:
		return graphdb.AllEdges
	}
	f := graphdb.AllEdges
	f.Refs = true
	return f
}

type stageRes struct {
	found      bool
	paths      []found
	exhaustive bool
	cut        bool
	visited    int
	hubTotal   int
	// the range the paths cover: the length of the shortest path (the sum for
	// the stretches of the --via points), how many stretches, and (the hub and
	// ref stages) the fewest hubs, and (the ref stage) the fewest references.
	shortHops, segs, hubs, refs int
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
		var short, hubs, refs, segs int
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
			if s >= 2 {
				ps, roots, best, c2, err := a.enumerateHub(append([]int64(nil), starts...), goals, want)
				if err != nil {
					return stageRes{}, err
				}
				c = c2
				for j := range ps {
					seg = append(seg, found{ps[j], roots[j]})
				}
				refs += best / refCost
				rest := best % refCost
				hubs += rest / hubCost
				short += rest % hubCost
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
				if next[p].refs() != next[q].refs() {
					return next[p].refs() < next[q].refs()
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
		return stageRes{found: true, paths: joined, exhaustive: true, cut: cut, visited: a.visited, hubTotal: len(a.hubs), shortHops: short, segs: segs, hubs: hubs, refs: refs}, nil
	}
}

type searchRes struct {
	stageRes
	// stage is the one that found (an index of stageNames); one past the last
	// stage searched if none did.
	stage    int
	searched []string
}

// search looks with the static edges, then (if there is no path) with the
// dynamic ones that have at most HubCandidates candidates, then (if there is
// still none) with every edge, and (if there is still none) with the references
// too (a function named as a value: not a call). With o.Hubs it begins with the
// third, with o.Refs with the fourth; o.NoRefs leaves the fourth out.
func (e *Engine) search(points [][]int64, o *Options, want int) (*searchRes, error) {
	last := 3
	if o.NoRefs {
		last = 2
	}
	s := 0
	if o.Hubs {
		s = 2
	}
	if o.Refs && !o.NoRefs {
		s = 3
	}
	var r stageRes
	for ; s <= last; s++ {
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
	return &searchRes{stageRes: r, stage: s, searched: append([]string(nil), stageNames[:min(s, last)+1]...)}, nil
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
		res.Exhaustive, res.EdgesExhausted = true, true
		res.ExhaustiveOf = exhaustiveOfFor(!o.NoRefs)
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
	refsSearched := len(sr.searched) > 0 && sr.searched[len(sr.searched)-1] == "ref"
	res.ExhaustiveOf = exhaustiveOfFor(refsSearched)
	res.Exhaustive = sr.exhaustive && !sr.cut
	res.EdgesExhausted = res.Exhaustive
	edgesDone := res.Exhaustive
	if sr.found && sr.stage == 1 {
		res.HubSkippedTotal = sr.hubTotal
	}
	if !sr.found {
		res.Certainty = ""
		if !sr.exhaustive {
			res.Warnings = append(res.Warnings, fmt.Sprintf("the search was cut (%d functions visited, at most %d hops): \"no path\" is not proved", sr.visited, MaxPathHops))
		}
		what := "no path by the static calls, nor by the dynamic calls (with at most " + fmt.Sprint(HubCandidates) + " candidates, or more)"
		if refsSearched {
			what += ", nor through references (a function named as a value)"
		}
		if o.filtered() {
			what += ", under the filters"
		}
		if edgesDone {
			what += ": none in the graph file" + map[bool]string{true: " (under the filters)", false: ""}[o.filtered()]
		}
		res.Hints = append(res.Hints, what)
		if o.filtered() {
			wf, err := e.withoutFilters(&asked, src, dst, startsFor, edgesDone)
			if err != nil {
				return nil, err
			}
			res.WithoutFilters = wf
			switch {
			case wf.Reachable && edgesDone:
				switch {
				case asked.nFilters() < 2:
					res.Hints = append(res.Hints, fmt.Sprintf("without the filters there is a path (%s, %d hops): the filter cuts it; take it off", wf.Certainty, wf.Hops))
				case len(wf.Culprits) > 0:
					res.Hints = append(res.Hints, fmt.Sprintf("without the filters there is a path (%s, %d hops): some of them cut it", wf.Certainty, wf.Hops))
					res.Hints = append(res.Hints, "taking off only one of these lets a path through (the others kept): "+strings.Join(wf.Culprits, "; "))
				default:
					res.Hints = append(res.Hints, fmt.Sprintf("without the filters there is a path (%s, %d hops), but taking off any one of them alone does not let one through: two or more of them block it (each may block a different path, or they block the same path together); take off more than one", wf.Certainty, wf.Hops))
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
			if refsSearched {
				res.Hints = append(res.Hints, "no entry reaches it by calls or by references (a function named as a value: a registered handler, a stored callback); a call by reflection, or from a framework or the runtime, is not in the graph file, so \"no path\" does not say the code is dead at run time (\"limits\")")
				res.Limits = entryLimitsRefs
			} else {
				res.Hints = append(res.Hints, "no entry reaches it by calls (the references were not searched: --no-refs): a function that is only passed as a value (a handler registered with a framework, a callback) is reached from the entry only through a call of a function value that lists it, or through a reference (the default search follows them); \"no path\" does not say the code is dead")
				res.Limits = entryLimits
			}
			// "no path" from the entries is not "not called at run time": a function that nothing
			// calls or names here may be called by code that was not read (a dependency, a
			// framework: an Unwrap that errors.Is calls, a Set that the flag package calls). What
			// the search proved of the edges is "edges_exhausted"
			res.Exhaustive = false
			for _, id := range dst.Funcs {
				if len(res.TargetFacts) >= 5 {
					break
				}
				f, err := e.fn(id)
				if err != nil {
					return nil, err
				}
				ft, err := e.db.FactsOf(id)
				if err != nil {
					return nil, err
				}
				recv := hasReceiver(ft.Lang, f.Symbol)
				if recv {
					res.Hints = append(res.Hints, fmt.Sprintf("%s is a method: a method that no call or reference in the file reaches may still be called through an interface from code that was not read (a dependency, a framework); \"no path\" does not say it is not called at run time", f.FID))
				}
				tf := TargetFact{ID: f.FID, Exported: ft.Exported, HasReceiver: recv, HasBody: ft.HasBody, Dep: ft.Dep, StaticCallSites: ft.StaticCallSites, DynamicCallSites: ft.DynamicCallSites}
				rs, err := e.db.RefsTo(id, 3)
				if err != nil {
					return nil, err
				}
				tf.ValueReferences = &rs.Total
				for _, ow := range rs.Owners {
					of, err := e.fn(ow.ID)
					if err != nil {
						return nil, err
					}
					tf.ValueReferenceOwners = append(tf.ValueReferenceOwners, RefOwner{Func: of.FID, Count: ow.Count})
				}
				res.TargetFacts = append(res.TargetFacts, tf)
			}
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
	for i := range res.Paths {
		if res.Paths[i].Relation != "calls" {
			res.Relation = res.Paths[i].Relation
		}
	}
	res.Listed = &Listed{Stage: stageNames[sr.stage], ShortestHops: sr.shortHops, MaxHops: sr.shortHops + PathSlack*sr.segs}
	if sr.stage >= 2 {
		h := sr.hubs
		res.Listed.Hubs = &h
	}
	if sr.stage == 3 {
		r := sr.refs
		res.Listed.Refs = &r
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
	case "reference":
		res.Hints = append(res.Hints,
			"no path by calls: these go through a reference (a step of kind \"ref\": a function holds the next one as a value, registered, stored or passed on; it is not a call). The paths listed go through the fewest references, then the fewest hubs, then the shortest",
			"\"at\" of a ref step is where the function is named (the registration), at the precision of its \"at_precision\": \"statement\" is the statement that holds the reference, not the argument or the element of a literal that names it (a table of routes has one place for all its rows): read the source of that statement to see which one; \"via_callee\" is the function that takes it as an argument. Whether the value is called is not known: the path says the function may be reached")
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

const exhaustiveOf = "the call edges in the graph file (calls, and the candidates of calls through interfaces and function values); not a function stored as a value, reflection, or a call from a framework"

// exhaustiveOfRefs is said when the references were searched too (the fourth stage).
const exhaustiveOfRefs = "the call edges in the graph file (calls, and the candidates of calls through interfaces and function values) and the references (a function named as a value: an argument, a stored function, a closure made there; the rule of sg.Program.Reachable); not reflection, or a call from a framework"

func exhaustiveOfFor(refs bool) string {
	if refs {
		return exhaustiveOfRefs
	}
	return exhaustiveOf
}

// entryLimits are what a path from the entries cannot see: the file holds
// calls, and the program reaches more at run time.
var entryLimits = []string{
	"a function stored as a value (a handler registered with a router: m.Get(\"/x\", f), a callback, a function put in a map or a struct) is not an edge of this search (--no-refs): its call is an edge only if a call of a function value lists it",
	"a call by reflection, and a call from a framework or the runtime (an override of a method of a dependency, such as ServeHTTP or a widget's build) is not an edge in the graph file",
	"so \"no path\" here is true of the call edges only: at run time the function may be reached (\"exhaustive\" is false, \"edges_exhausted\" says the search of the edges was complete); \"target_facts\" says what the file knows of it (\"has_receiver\": it is a method)",
}

// entryLimitsRefs are what a path from the entries cannot see when the
// references were followed: the calls and the references are edges, and the
// program reaches more at run time.
var entryLimitsRefs = []string{
	"a call by reflection, and a call from a framework or the runtime (an override of a method of a dependency, such as ServeHTTP or a widget's build) is not an edge in the graph file",
	"a reference is a function named as a value; whether the value is called is not known, so a path through one says \"may be reached\". A function named in a string (a route by name, a plugin by path) is not a reference",
	"so \"no path\" here is true of the calls and the references (the same rule as sg.Program.Reachable); \"edges_exhausted\" is true when the search of them was complete; \"exhaustive\" is always false from the entries (a function of the code that was read can be called by code that was not read: an interface method a dependency calls); \"target_facts\" says what the file knows of the goal (\"has_receiver\": it is a method)",
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
	plain := Options{Limit: 1, Hubs: o.Hubs, Refs: o.Refs, NoRefs: o.NoRefs}
	sr, err := e.search([][]int64{starts, dst.Funcs}, &plain, 1)
	if err != nil {
		return nil, err
	}
	wf := &WithoutFilters{Reachable: sr.found, Searched: sr.searched, Exhaustive: sr.exhaustive && !sr.cut}
	if sr.found {
		wf.Certainty = certaintyOf(sr.paths[0].edges)
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
	st.AtUnknown = st.At == ""
	if f.Symbol != f.FID {
		st.Symbol = f.Symbol
	}
	if dynamic(c.Kind) {
		st.Candidates = c.NCand
		st.Hub = c.NCand > HubCandidates
	}
	return st, nil
}

// certaintyOf is the certainty of a path of edges: certain (every call is named),
// uncertain (a dynamic call), hub (a call of many candidates), reference (a
// function named as a value: not a call).
func certaintyOf(edges []graphdb.Edge) string {
	c := "certain"
	for i := range edges {
		switch {
		case isRef(&edges[i]):
			return "reference"
		case isHub(&edges[i]):
			c = "hub"
		case dynamic(edges[i].Kind) && c == "certain":
			c = "uncertain"
		}
	}
	return c
}

// refNote is what a step of kind ref says.
const refNote = "this function holds the next one as a value (registered, stored, passed on); it is not a call"

// refPathNote says how far a path with a reference is to be trusted.
const refPathNote = "a path with a reference is a wide approximation of \"may be reached\": a function that is registered or stored may be called or not (the graph does not say), and the steps of kind ref are not calls"

func (e *Engine) pathOut(start int64, edges []graphdb.Edge, prefix []Step, suffix *Step) (Path, error) {
	p := Path{Certainty: "certain", Relation: "calls"}
	p.Steps = append(p.Steps, prefix...)
	cur := start
	// raise the certainty of the path: certain, uncertain (a dynamic call), hub (a call of many candidates)
	raise := func(kind string, hub bool) {
		if p.Certainty == "reference" {
			return
		}
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
			st.AtUnknown = st.At == ""
			st.Kind = ed.Kind
			if dynamic(ed.Kind) {
				st.Candidates = ed.NCand
			}
			if isHub(ed) {
				st.Hub = true
			}
			if isRef(ed) {
				st.RefKind, st.ViaCallee, st.Note, st.AtPrecision = ed.RefKind, ed.Via, refNote, "statement"
				p.Certainty, p.Relation, p.Note = "reference", "calls+refs", refPathNote
				p.Refs++
			} else {
				raise(ed.Kind, st.Hub)
				if st.Sites, err = e.siteIDs(ed.CallSite); err != nil {
					return err
				}
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

// hasReceiver reads from the symbol whether the function is a method: Go
// writes "(*pkg.T).M" or "(pkg.T).M", Dart "uri#Class.m", Rust "<T as Tr>::m" or
// "path::Type::m" (a type name starts with a capital).
func hasReceiver(lang, symbol string) bool {
	switch lang {
	case "go":
		return strings.HasPrefix(symbol, "(")
	case "dart":
		if i := strings.LastIndex(symbol, "#"); i >= 0 {
			return strings.Contains(symbol[i+1:], ".")
		}
	case "rust":
		if strings.HasPrefix(symbol, "<") {
			return true
		}
		parts := strings.Split(symbol, "::")
		if n := len(parts); n >= 2 && parts[n-2] != "" {
			c := parts[n-2][0]
			return c >= 'A' && c <= 'Z'
		}
	}
	return false
}
