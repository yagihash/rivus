package graphq

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/graphdb"
)

// TopPackages is how many packages by_package lists.
const TopPackages = 10

// Impl is a function that some call site through the interface method lists as
// a candidate.
type Impl struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol,omitempty"`
	At     string `json:"at,omitempty"`
	NoBody bool   `json:"no_body,omitempty"`
	Dep    bool   `json:"dep,omitempty"`
	// CallSites is how many call sites of the method list it (of those the filters let pass).
	CallSites int `json:"call_sites"`
}

// MethodChoice is an interface method a short name may stand for.
type MethodChoice struct {
	ID        string `json:"id"`
	CallSites int    `json:"call_sites"`
}

// PackageCount is a count of a package (or the directory of a file).
type PackageCount struct {
	Package string `json:"package"`
	Count   int    `json:"count"`
}

// ImplsResult is the answer of impls.
type ImplsResult struct {
	Common
	// Method is the interface method, as the call sites name it (the static type of the call).
	Method string `json:"method,omitempty"`
	// Basis says what the list is: the union of the candidates of the call
	// sites in the graph, not every type that implements the interface.
	Basis string `json:"basis"`
	// CallSites is the number of call sites through the method in the graph;
	// ZeroCandidateSites those of them that have no candidate (the analysis
	// found no type for the receiver: their implementations are not listed).
	CallSites          int `json:"call_sites"`
	ZeroCandidateSites int `json:"zero_candidate_sites"`
	// Reason says why there is nothing to list, when no call site names the method.
	Reason string `json:"reason,omitempty"`
	// ImplsTotal is the number of implementations (Truncated says how many are not listed).
	ImplsTotal     int            `json:"impls_total"`
	Impls          []Impl         `json:"impls"`
	ByPackage      []PackageCount `json:"by_package,omitempty"`
	ByPackageOther int            `json:"by_package_other,omitempty"`
	Choices
}

const basisGo = "the union, over the call sites through this interface method that are in the graph, of the candidates the analysis (VTA) left for each call site: " +
	"a type that no call site can reach is not listed, so this is not every type that implements the interface"
const basisOther = "the union, over the call sites through this interface method that are in the graph, of the candidates the extractor of the language gave for each call site: " +
	"a type that no call site can reach is not listed, so this is not every type that implements the interface"

// packageOf is the package of a function: for Go the import path in the symbol,
// else the directory of the file it is declared in.
func packageOf(f *graphdb.Func) string {
	if f.Lang == "go" {
		s := f.Symbol
		if strings.HasPrefix(s, "(") {
			if i := strings.Index(s, ")"); i > 0 {
				s = s[1:i]
			}
		}
		s = strings.TrimLeft(s, "*")
		if i := strings.Index(s, "["); i >= 0 {
			s = s[:i]
		}
		if !strings.ContainsAny(s, "{ ") {
			base := strings.LastIndex(s, "/") + 1
			if j := strings.Index(s[base:], "."); j > 0 {
				return s[:base+j]
			}
		}
	}
	if f.File != "" {
		return path.Dir(f.File)
	}
	return "(no position)"
}

// resolveMethod finds the interface method a specification names: the name as
// the call sites have it, or its end (at the boundary of an identifier) when
// only one method ends so. Several are returned as choices.
func (e *Engine) resolveMethod(spec string) (graphdb.MethodRow, []graphdb.MethodRow, error) {
	m, err := e.db.InterfaceMethod(spec)
	if err != nil || m.Sites > 0 {
		return m, nil, err
	}
	all, err := e.db.InterfaceMethods()
	if err != nil {
		return m, nil, err
	}
	rows := make([]graphdb.SymRow, len(all))
	for i, r := range all {
		rows[i] = graphdb.SymRow{ID: int64(i), FID: r.Callee, Symbol: r.Callee, HasBody: true}
	}
	_, matched := matchEnd(rows, spec)
	var hits []graphdb.MethodRow
	for _, r := range matched {
		hits = append(hits, all[r.ID])
	}
	if len(hits) == 1 {
		return hits[0], nil, nil
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Sites != hits[j].Sites {
			return hits[i].Sites > hits[j].Sites
		}
		return hits[i].Callee < hits[j].Callee
	})
	return graphdb.MethodRow{}, hits, nil
}

func (e *Engine) noViaFilter(op string, o *Options) error {
	if len(o.Via) > 0 {
		return fmt.Errorf("--via does not apply to %s (it lists the functions and call sites that touch the target; use --avoid or --only-impl)", op)
	}
	return nil
}

// Impls answers which functions the call sites through an interface method
// may reach.
func (e *Engine) Impls(target string, o Options) (*ImplsResult, error) {
	o.normalize(DefaultLimit)
	if err := e.noViaFilter("impls", &o); err != nil {
		return nil, err
	}
	res := &ImplsResult{Common: e.common("impls", o), Impls: []Impl{}}
	res.Query.Target = target
	res.Truncated = Truncated{Limit: o.Limit, Unit: "impls"}
	res.Basis = basisOther
	if err := e.prepare(&o, &res.Common); err != nil {
		return nil, err
	}
	m, choices, err := e.resolveMethod(target)
	if err != nil {
		return nil, err
	}
	if len(choices) > 1 {
		res.Query.Resolved = fmt.Sprintf("%d interface methods end with %q", len(choices), target)
		for i, c := range choices {
			if i >= MaxChoices {
				break
			}
			res.Ambiguous = append(res.Ambiguous, MethodChoice{ID: c.Callee, CallSites: c.Sites})
		}
		res.AmbiguousTotal = len(choices)
		res.Hints = append(res.Hints, "several interface methods end with that name: ask again with the \"id\" of one")
		if len(choices) > MaxChoices {
			res.Hints = append(res.Hints, fmt.Sprintf("%d end with it, %d are listed (the most called first): qualify the name with the interface (pkg.Iface.Method)", len(choices), MaxChoices))
		}
		return res, nil
	}
	if m.Sites == 0 {
		return e.noCallSites(res, target)
	}
	res.Method = m.Callee
	res.Query.Resolved = "interface method " + m.Callee
	if m.Callee != target {
		res.Query.Resolved += fmt.Sprintf(" (the name %q is the end of it, and no other interface method ends so)", target)
	}
	res.CallSites, res.ZeroCandidateSites = m.Sites, m.Zero
	if m.Go > 0 {
		res.Basis = basisGo
	}
	edges, err := e.db.InterfaceEdges(m.Callee)
	if err != nil {
		return nil, err
	}
	perFunc := map[int64]int{}
	for i := range edges {
		ed := &edges[i]
		if !o.pass(ed) {
			continue
		}
		perFunc[ed.To]++
	}
	type row struct {
		f *graphdb.Func
		n int
	}
	var rows []row
	for id, n := range perFunc {
		f, err := e.fn(id)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row{f, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.f.Dep != b.f.Dep {
			return !a.f.Dep
		}
		if a.n != b.n {
			return a.n > b.n
		}
		return a.f.FID < b.f.FID
	})
	res.ImplsTotal = len(rows)
	pk := map[string]int{}
	for i, r := range rows {
		pk[packageOf(r.f)]++
		if i >= o.Limit {
			continue
		}
		im := Impl{ID: r.f.FID, NoBody: !r.f.HasBody, Dep: r.f.Dep, CallSites: r.n}
		if r.f.Symbol != r.f.FID {
			im.Symbol = r.f.Symbol
		}
		if r.f.File != "" && r.f.Line > 0 {
			im.At = pos(r.f.File, r.f.Line, r.f.Col)
		}
		res.Impls = append(res.Impls, im)
	}
	if len(rows) > o.Limit {
		res.Truncated.Count = len(rows) - o.Limit
		var pcs []PackageCount
		for p, n := range pk {
			pcs = append(pcs, PackageCount{p, n})
		}
		sort.Slice(pcs, func(i, j int) bool {
			if pcs[i].Count != pcs[j].Count {
				return pcs[i].Count > pcs[j].Count
			}
			return pcs[i].Package < pcs[j].Package
		})
		if len(pcs) > TopPackages {
			for _, p := range pcs[TopPackages:] {
				res.ByPackageOther += p.Count
			}
			pcs = pcs[:TopPackages]
		}
		res.ByPackage = pcs
		res.Hints = append(res.Hints, fmt.Sprintf("%d of %d implementations are listed: by_package counts all of them by package; raise --limit, or ask dispatchers of one implementation", o.Limit, len(rows)))
	}
	res.Hints = append(res.Hints, "this is the union of the candidates of the call sites in the graph (\"basis\"), not every type that implements the interface: a type no call site can reach is missing")
	if m.Zero > 0 {
		res.Hints = append(res.Hints, fmt.Sprintf("%d of the %d call sites have no candidate (zero_candidate_sites): the analysis found no type for their receivers, so what they may call is not listed", m.Zero, m.Sites))
	}
	if len(rows) == 0 && o.filtered() {
		res.Hints = append(res.Hints, "the filters above leave no implementation: without them there are more")
	} else if o.filtered() {
		res.Hints = append(res.Hints, "the list is narrowed by the filters above (\"filters\"): without them it has more")
	}
	if len(rows) == 0 && m.Zero == m.Sites {
		res.Reason = "every call site through this method has no candidate: the analysis found no implementation for any of them"
	}
	return res, nil
}

// noCallSites answers a method that no call site names: an empty list, and why.
func (e *Engine) noCallSites(res *ImplsResult, target string) (*ImplsResult, error) {
	tg, err := e.Resolve(target, false)
	if err != nil {
		return nil, err
	}
	res.Query.Resolved = tg.Desc
	res.Reason = "no call site through an interface names this method in the graph, so there is no candidate to list: the program does not call it through the interface " +
		"(or the call is in code that was not read), or the name is not an interface method"
	res.Hints = append(res.Hints, "impls lists what the call sites in the graph reach; with no call site it can say nothing (an empty list is not \"no implementation\")")
	if !tg.isAmbiguous() && tg.Call == nil && len(tg.Funcs) > 0 {
		if f, err := e.fn(tg.Funcs[0]); err == nil && f.HasBody {
			res.Hints = append(res.Hints, "the name is a function with a body: if it implements an interface method, ask dispatchers of it for the call sites that may call it")
		}
	}
	return res, nil
}

// DispatchSite is a dynamic call that lists the function as a candidate.
type DispatchSite struct {
	At         string `json:"at"`
	In         string `json:"in"`
	Kind       string `json:"kind"`
	Candidates int    `json:"candidates"`
	// Callee is the static type of the call: the interface method it goes
	// through (for a call of a function value, the name of the value).
	Callee string   `json:"callee,omitempty"`
	Sites  []string `json:"sites,omitempty"`
}

// CalleeCount is a count of the call sites that name a callee.
type CalleeCount struct {
	Callee    string `json:"callee"`
	CallSites int    `json:"call_sites"`
}

// DispatchersResult is the answer of dispatchers.
type DispatchersResult struct {
	Common
	Start []string `json:"start"`
	Basis string   `json:"basis"`
	// CallSitesTotal is the number of dynamic call sites that list the function.
	CallSitesTotal int            `json:"call_sites_total"`
	CallSites      []DispatchSite `json:"call_sites"`
	// ByCallee counts them by the interface method they go through (the top
	// TopPackages; ByCalleeOther is the count of the rest).
	ByCallee      []CalleeCount `json:"by_callee,omitempty"`
	ByCalleeOther int           `json:"by_callee_other,omitempty"`
	Choices
}

const dispatchBasis = "the dynamic call sites (through an interface or a function value) in the graph that list this function as one of their candidates: " +
	"the analysis narrows the candidates of each call site, so a call that it did not connect to this function is not here"

// Dispatchers answers which dynamic calls may reach a function, and through
// which interface method.
func (e *Engine) Dispatchers(target string, o Options) (*DispatchersResult, error) {
	o.normalize(DefaultLimit)
	if err := e.noViaFilter("dispatchers", &o); err != nil {
		return nil, err
	}
	tg, err := e.Resolve(target, o.IncludeBuiltin)
	if err != nil {
		return nil, err
	}
	res := &DispatchersResult{Common: e.common("dispatchers", o), Start: []string{}, CallSites: []DispatchSite{}, Basis: dispatchBasis}
	res.Query.Target, res.Query.Resolved = target, tg.Desc
	res.Truncated = Truncated{Limit: o.Limit, Unit: "call sites"}
	if err := e.prepare(&o, &res.Common); err != nil {
		return nil, err
	}
	if tg.isAmbiguous() {
		c, err := e.choices(tg)
		if err != nil {
			return nil, err
		}
		res.Choices = c
		res.Hints = append(res.Hints, ambiguousHints(tg, c)...)
		return res, nil
	}
	if tg.Call != nil {
		return nil, &SpecError{Spec: target, Msg: "dispatchers takes a function (an implementation), not a call"}
	}
	// a symbol that several functions have: the method, and the wrappers of the types that embed it
	// (their symbol is the embedded method's). The call sites that list a wrapper are not the ones
	// that list the method, so the method (the function whose ID is the symbol) is what is asked.
	funcs := tg.Funcs
	wrappers := 0
	if len(funcs) > 1 {
		for _, id := range funcs {
			if f, err := e.fn(id); err == nil && f.FID == f.Symbol {
				wrappers = len(funcs) - 1
				funcs = []int64{id}
				break
			}
		}
	}
	type site struct {
		DispatchSite
		id int64
	}
	var all []site
	seen := map[int64]bool{}
	for _, id := range funcs {
		f, err := e.fn(id)
		if err != nil {
			return nil, err
		}
		res.Start = append(res.Start, f.FID)
		edges, err := e.db.DynamicInEdges(id)
		if err != nil {
			return nil, err
		}
		for i := range edges {
			ed := &edges[i]
			if !o.pass(&ed.Edge) || seen[ed.CallSite] {
				continue
			}
			seen[ed.CallSite] = true
			from, err := e.fn(ed.From)
			if err != nil {
				return nil, err
			}
			sites, err := e.siteIDs(ed.CallSite)
			if err != nil {
				return nil, err
			}
			all = append(all, site{DispatchSite{At: pos(ed.File, ed.Line, ed.Col), In: from.FID, Kind: ed.Kind, Candidates: ed.NCand, Callee: ed.Callee, Sites: sites}, ed.CallSite})
		}
	}
	// the calls that name the function most closely first: few candidates, then by place
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Candidates != all[j].Candidates {
			return all[i].Candidates < all[j].Candidates
		}
		return all[i].At < all[j].At
	})
	res.CallSitesTotal = len(all)
	byCallee := map[string]int{}
	for i, s := range all {
		byCallee[s.Callee]++
		if i < o.Limit {
			res.CallSites = append(res.CallSites, s.DispatchSite)
		}
	}
	if len(all) > o.Limit {
		res.Truncated.Count = len(all) - o.Limit
	}
	var cc []CalleeCount
	for c, n := range byCallee {
		cc = append(cc, CalleeCount{c, n})
	}
	sort.Slice(cc, func(i, j int) bool {
		if cc[i].CallSites != cc[j].CallSites {
			return cc[i].CallSites > cc[j].CallSites
		}
		return cc[i].Callee < cc[j].Callee
	})
	if len(cc) > TopPackages {
		for _, c := range cc[TopPackages:] {
			res.ByCalleeOther += c.CallSites
		}
		cc = cc[:TopPackages]
	}
	res.ByCallee = cc
	if len(all) == 0 {
		res.Hints = append(res.Hints, "no dynamic call site lists this function as a candidate: it may be called only by name (callers), or not at all in the code that was read")
	} else {
		res.Hints = append(res.Hints, "\"callee\" is the interface method each call goes through: impls <callee> lists what else it may reach; callers <this function> adds the static calls")
		if res.Truncated.Count > 0 {
			res.Hints = append(res.Hints, fmt.Sprintf("%d of %d call sites are listed (the fewest candidates first): by_callee counts all of them by interface method; raise --limit", o.Limit, len(all)))
		}
	}
	if wrappers > 0 {
		res.Hints = append(res.Hints, fmt.Sprintf("%d other functions have this symbol (wrappers of the types that embed it): only the call sites that list the method itself are here; ask with the ID of a wrapper (callers lists them) for the others", wrappers))
	} else if len(funcs) > 1 {
		res.Hints = append(res.Hints, fmt.Sprintf("%d functions have this symbol: the call sites of all of them are listed", len(funcs)))
	}
	if o.filtered() {
		res.Hints = append(res.Hints, "the answer is narrowed by the filters above (\"filters\"): without them it has more")
	}
	return res, nil
}
