package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/model"
)

// Facts are the rows and the `rejected:` entries of the model files that a candidate is matched with.
type Facts struct {
	// Files are the model files that have a row or a rejected entry of the package (the first one is where new entries go).
	Files []string
	// by symbol: the file of the row
	Sinks, Sources, Sanitizers, Flows, Calls map[string]string
	SourceTypes                              map[string]string  // param_type sources: the type
	SourceFieldPrefix                        map[string]string  // field sources: "pkg.Type.*" or "pkg.Type.Field"
	SinkRows                                 []model.Sink       // of the package, for the recall
	SourceRows                               []model.Source     // of the package
	FlowRows                                 []model.Propagator // of the package: the rows with flows (the skeleton reads them)
	Rejected                                 []RejectedEntry    // of the package
	Deferred                                 []DeferredEntry    // of the package
	BodyReplaced                             []string           // the packages that the files declare (body_replaced_packages)
}

// RejectedEntry is a `rejected:` entry and the file it is in.
type RejectedEntry struct {
	model.Rejected
	File string
	// Wild: the symbol was written with /v* (every major): it can name something that only another major has, so it is never stale.
	Wild bool
}

// DeferredEntry is a `deferred:` entry and the file it is in.
type DeferredEntry struct {
	model.Deferred
	File string
	Wild bool
}

// inPackage: the symbol (function, method, field, type) is of the package.
func inPackage(sym, pkg string) bool {
	s := strings.TrimPrefix(sym, "(")
	s = strings.TrimPrefix(s, "*")
	if isDart(pkg) {
		// package:dio/src/dio.dart#Dio.get is of package:dio (and not of package:dio_foo)
		return strings.HasPrefix(s, pkg+"/")
	}
	return strings.HasPrefix(s, pkg+".")
}

// BuiltinFiles lists the built-in model files of a checkout.
func BuiltinFiles(repo string) ([]string, error) {
	fs, err := filepath.Glob(filepath.Join(repo, "internal", "model", "builtin", "*.yaml"))
	sort.Strings(fs)
	return fs, err
}

var majorElemRe = regexp.MustCompile(`/v([2-9][0-9]*|[1-9][0-9]+)(/|$)`)

// majorOf is the major of a Go module path as the path writes it ("93" for `x/v93/pkg`, "" if there is none).
func majorOf(pkg string) string {
	if m := majorElemRe.FindStringSubmatch(pkg); m != nil {
		return m[1]
	}
	return ""
}

// LoadFacts reads the model files with the loader of the model (a file that is not valid, a `rejected:` without `why`, is an error).
// files is the list to read: the built-in files of the checkout, or the one that -model names.
func LoadFacts(files []string, pkg string) (*Facts, error) {
	f := &Facts{Sinks: map[string]string{}, Sources: map[string]string{}, Sanitizers: map[string]string{}, Flows: map[string]string{},
		Calls: map[string]string{}, SourceTypes: map[string]string{}, SourceFieldPrefix: map[string]string{}}
	used := map[string]bool{}
	// A row written with /v* (any major of the module, ADR-110) is for the major that is checked: it is read as the concrete symbol of
	// that major. A row of another major is not of this package, so what only another major has is never taken for a vanished candidate.
	major := majorOf(pkg)
	conc := func(s string) string {
		if major != "" && model.HasMajorWildcard(s) {
			return model.ConcreteMajor(s, major)
		}
		return s
	}
	for _, path := range files {
		fh, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		rs, err := model.Parse(fh, "builtin:"+filepath.Base(path))
		fh.Close()
		if err != nil {
			return nil, err
		}
		for _, s := range rs.Sinks {
			s.Symbol = conc(s.Symbol)
			f.Sinks[s.Symbol] = path
			if inPackage(s.Symbol, pkg) {
				f.SinkRows = append(f.SinkRows, s)
				used[path] = true
			}
		}
		for _, s := range rs.Sources {
			s.Symbol, s.Type, s.Field = conc(s.Symbol), conc(s.Type), conc(s.Field)
			if s.Symbol != "" {
				f.Sources[s.Symbol] = path
			}
			if s.Type != "" {
				f.SourceTypes[strings.TrimPrefix(s.Type, "*")] = path
			}
			if s.Field != "" {
				f.SourceFieldPrefix[s.Field] = path
			}
			if inPackage(s.Symbol, pkg) || inPackage(s.Type, pkg) || inPackage(s.Field, pkg) {
				f.SourceRows = append(f.SourceRows, s)
				used[path] = true
			}
		}
		for _, s := range rs.Sanitizers {
			f.Sanitizers[conc(s.Symbol)] = path
		}
		for _, p := range rs.Propagators {
			p.Symbol = conc(p.Symbol)
			f.Flows[p.Symbol] = path
			if len(p.Calls) > 0 {
				f.Calls[p.Symbol] = path
			}
			if inPackage(p.Symbol, pkg) {
				f.FlowRows = append(f.FlowRows, p)
				used[path] = true
			}
		}
		for _, b := range rs.BodyReplaced {
			f.BodyReplaced = append(f.BodyReplaced, b.Package)
		}
		for _, d := range rs.Deferred {
			wild := model.HasMajorWildcard(d.Symbol)
			d.Symbol = conc(d.Symbol)
			if inPackage(d.Symbol, pkg) {
				f.Deferred = append(f.Deferred, DeferredEntry{d, path, wild})
				used[path] = true
			}
		}
		for _, r := range rs.Rejected {
			wild := model.HasMajorWildcard(r.Symbol)
			r.Symbol = conc(r.Symbol)
			if inPackage(r.Symbol, pkg) {
				f.Rejected = append(f.Rejected, RejectedEntry{r, path, wild})
				used[path] = true
			}
		}
	}
	for _, p := range files {
		if used[p] {
			f.Files = append(f.Files, p)
		}
	}
	return f, nil
}

// BodiesRead says whether the analysis reads the bodies of the package: not for the standard library, and not for a package that a
// model file declares in body_replaced_packages (exactly, or under a `/...`).
func (f *Facts) BodiesRead(pkg string) bool {
	if !isDart(pkg) && isStdlib(pkg) {
		return false
	}
	for _, b := range f.BodyReplaced {
		if b == pkg || strings.HasSuffix(b, model.Subpackages) && strings.HasPrefix(pkg+"/", strings.TrimSuffix(b, model.Subpackages)+"/") {
			return false
		}
	}
	return true
}

// receiverType: `(*a/b.T).M` is `a/b.T`; "" for a function.
func receiverType(sym string) string {
	if strings.HasPrefix(sym, "package:") {
		// Dart: package:a/b.dart#Type.member is of package:a/b.dart#Type
		if h := strings.Index(sym, "#"); h > 0 {
			if d := strings.Index(sym[h:], "."); d > 0 {
				return sym[:h+d]
			}
		}
		return ""
	}
	if !strings.HasPrefix(sym, "(") {
		return ""
	}
	end := strings.Index(sym, ")")
	if end < 0 {
		return ""
	}
	return strings.TrimPrefix(sym[1:end], "*")
}

// rowFor says which file has a row that decides the candidate ("" if none): the row of its kind, or a row of flows (a function that
// a flow says something about has been looked at).
func (f *Facts) rowFor(it Item) string {
	switch it.Kind {
	case KindSink:
		return f.Sinks[it.Symbol]
	case KindSource:
		if p := f.Sources[it.Symbol]; p != "" {
			return p
		}
		if p := f.Flows[it.Symbol]; p != "" {
			return p
		}
		if rt := receiverType(it.Symbol); rt != "" {
			if p := f.SourceTypes[rt]; p != "" {
				return p
			}
		}
		if p := f.SourceFieldPrefix[it.Symbol]; p != "" {
			return p
		}
		// "pkg.Type.*"
		if i := strings.LastIndex(it.Symbol, "."); i > 0 {
			return f.SourceFieldPrefix[it.Symbol[:i]+".*"]
		}
	case KindSanitizer:
		if p := f.Sanitizers[it.Symbol]; p != "" {
			return p
		}
		return f.Flows[it.Symbol]
	case KindCalls:
		return f.Calls[it.Symbol]
	}
	return ""
}

// decidedBy says whether a list of entries (symbol, kind, position) decides the candidate: an entry of the symbol and kind with no
// position, or entries that together name every position of the candidate. any says whether at least one entry matched.
func decidedBy(it Item, n int, at func(i int) (sym, kind, pos string)) (all, any bool) {
	left := map[string]bool{}
	for _, p := range it.Positions {
		left[p] = true
	}
	for i := 0; i < n; i++ {
		sym, kind, pos := at(i)
		if sym != it.Symbol || kind != it.Kind {
			continue
		}
		any = true
		if pos == "" {
			return true, true
		}
		delete(left, pos)
	}
	return len(left) == 0 && any, any
}

// Matched is the result of the matching.
type Matched struct {
	ByRow, ByRejected, ByDeferred, Undecided []Item
	// Stale are the `rejected:` entries whose symbol is no longer in the API of the package.
	Stale []RejectedEntry
	// StaleDeferred: the same for `deferred:`.
	StaleDeferred []DeferredEntry
	// NoLongerCandidate are the `rejected:` entries whose symbol is in the API but is not a candidate of the rule table (any more): a note, not a warning.
	NoLongerCandidate []RejectedEntry
}

// Match sorts the candidates (layer 1) into the ones that have a row, the ones that `rejected:` and `deferred:` decide (together, place by
// place; one that has any `deferred:` place is a deferred one), and the undecided ones.
func Match(api *API, f *Facts) *Matched {
	m := &Matched{}
	for _, it := range api.Items {
		if f.rowFor(it) != "" {
			m.ByRow = append(m.ByRow, it)
			continue
		}
		var cov []model.Rejected
		for _, r := range f.Rejected {
			cov = append(cov, r.Rejected)
		}
		for _, d := range f.Deferred {
			cov = append(cov, model.Rejected{Symbol: d.Symbol, Kind: d.Kind, Position: d.Position})
		}
		all, _ := decidedBy(it, len(cov), func(i int) (string, string, string) { return cov[i].Symbol, cov[i].Kind, cov[i].Position })
		if !all {
			m.Undecided = append(m.Undecided, it)
			continue
		}
		_, anyDef := decidedBy(it, len(f.Deferred), func(i int) (string, string, string) {
			return f.Deferred[i].Symbol, f.Deferred[i].Kind, f.Deferred[i].Position
		})
		if anyDef {
			m.ByDeferred = append(m.ByDeferred, it)
		} else {
			m.ByRejected = append(m.ByRejected, it)
		}
	}
	for _, r := range f.Rejected {
		if _, ok := api.Symbols[r.Symbol]; !ok && !r.Wild {
			m.Stale = append(m.Stale, r)
		}
	}
	cand := map[string]bool{}
	for _, it := range api.Items {
		cand[it.Kind+"\x00"+it.Symbol] = true
	}
	for _, r := range f.Rejected {
		if _, ok := api.Symbols[r.Symbol]; ok && !cand[r.Kind+"\x00"+r.Symbol] {
			m.NoLongerCandidate = append(m.NoLongerCandidate, r)
		}
	}
	for _, d := range f.Deferred {
		if _, ok := api.Symbols[d.Symbol]; !ok && !d.Wild {
			m.StaleDeferred = append(m.StaleDeferred, d)
		}
	}
	return m
}

// UndecidedYAML is the list of the undecided candidates as entries of `rejected:` with `why` empty: the person who decides moves an
// entry to the model file with a reason, or makes a row for it (an entry without a reason is refused by the loader).
func UndecidedYAML(pkg string, undecided []Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# The undecided candidates of layer 1 of %s. For each: add a row to the model, or move the entry to `rejected:` of the model file with a reason (`why`).\n", pkg)
	b.WriteString("# This file is a work file: it is not committed.\n")
	b.WriteString("rejected:\n")
	for _, it := range undecided {
		fmt.Fprintf(&b, "  # %s (%s): %s\n", it.Rule, strings.Join(it.Positions, ", "), it.Sig)
		fmt.Fprintf(&b, "  - symbol: %s\n    kind: %s\n    why: \"\"\n", yamlScalar(it.Symbol), it.Kind)
	}
	return b.String()
}

// yamlScalar quotes a symbol when YAML would read it as something else (a symbol that starts with `(` or `*` is a plain scalar for
// YAML, but is quoted anyway to be safe).
func yamlScalar(s string) string {
	if strings.ContainsAny(s[:1], "*&!|>'\"%@`[]{}#") || strings.Contains(s, ": ") || strings.Contains(s, " #") {
		return "\"" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + "\""
	}
	return s
}
