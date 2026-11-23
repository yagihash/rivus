package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/yagihash/rivus/internal/model"
)

// The expansion of bulk adoption (docs/DESIGN-package-models.md, 6.12; docs/MODELING.md, 4c-2): the decisions of `adopt_rules:` of the hand file
// become rows (`origin: generated`, `decision: <id>`) in `<hand file>.adopt.gen.yaml`. The rows are a function of (the decisions, the hand rows,
// `rejected:` and `deferred:`, the version of the package, the rule table): no time, no randomness, so the same input gives the same bytes.
//
// A position that a hand row has, or that `rejected:` or `deferred:` decides, is not generated (the hand row wins). Two decisions that claim the
// same position are joined by ResolveClaims. `covers` is compared with what the decision claims now (a growth stops the command: exit 70).

// GeneratorVersion is the version of the expansion that the header of the file records (`generator: model-author@N`).
const GeneratorVersion = "1"

// GenRow is a row of the generated file.
type GenRow struct {
	Kind      string // sink | source
	Symbol    string // concrete (the major that is checked)
	Args      []int
	Parts     []GenPart
	Category  string
	Labels    []string
	Field     bool // source: a `kind: field` row (the symbol is the field)
	Decisions []string
}

// GenPart is a field of an argument: `arg1.Object`.
type GenPart struct {
	Arg   int
	Field string
}

// DecisionCount is what a decision did at this run.
type DecisionCount struct {
	ID       string
	Covers   int
	Claimed  []string // symbols that the decision claims (before the hand rows and rejected: take their share), sorted
	ByHand   []string // of them: symbols with a position that a hand row has
	ByReject []string // of them: symbols with a position that rejected: or deferred: decides
	Rows     int      // generated rows that name the decision
	// New are the claimed symbols that were neither in the previous file of generated rows nor have a hand row: what a growth of covers is.
	New []string
	// Representatives
	RepsMissing []string // named, but not a generated row of this decision
	RepsAdvise  []string // symbols that would be the representatives of the combinations that have none
}

// ExpandResult is what Expand computes.
type ExpandResult struct {
	Rows            []GenRow
	Warnings        []string
	Counts          []DecisionCount
	HandCovered     int      // hand rows (sink and source, by symbol) of the package that a decision claims
	HandTotal       int      // hand rows of the package that a decision could claim (sinks, and sources by symbol or field)
	HandNotCovered  []string // the symbols (with positions) of the others
	ConflictErr     error
	VocabularyError error
}

// ---- the scan of a decision ----

func recvOfItem(it Item) string {
	if len(it.Positions) == 1 && it.Positions[0] == "field" {
		if i := strings.LastIndex(it.Symbol, "."); i > 0 {
			return it.Symbol[:i]
		}
		return ""
	}
	return receiverType(it.Symbol)
}

func lastName(sym string) string {
	if i := strings.LastIndex(sym, "."); i >= 0 {
		return sym[i+1:]
	}
	return sym
}

// inScope applies the scope of a decision (all keys that are set, AND) to a candidate. The package is the one of the run (the decision was kept
// by LoadFacts for it); a symbol of another package, as `recv.Field` of a type of a dependency, cannot be in the API.
func inScope(sc model.AdoptScope, it Item) bool {
	if len(sc.Functions) > 0 {
		ok := false
		for _, f := range sc.Functions {
			ok = ok || f == lastName(it.Symbol)
		}
		if !ok {
			return false
		}
	}
	if len(sc.ReceiverTypes) == 0 && sc.ReceiverSuffix == "" && sc.ReceiverPrefix == "" {
		return true
	}
	rt := recvOfItem(it)
	if rt == "" {
		return false
	}
	if len(sc.ReceiverTypes) > 0 {
		ok := false
		for _, t := range sc.ReceiverTypes {
			ok = ok || t == rt
		}
		if !ok {
			return false
		}
	}
	name := lastName(rt)
	if sc.ReceiverSuffix != "" && !strings.HasSuffix(name, sc.ReceiverSuffix) {
		return false
	}
	if sc.ReceiverPrefix != "" && !strings.HasPrefix(name, sc.ReceiverPrefix) {
		return false
	}
	return true
}

func wordsMatch(d Decision, t Tag) bool {
	if len(d.Words) == 0 {
		return true
	}
	for _, w := range t.Words {
		for _, x := range d.Words {
			if w == x {
				return true
			}
		}
	}
	return false
}

// checkVocabulary refuses a word that is not in the vocabulary of the rule of the decision (the loader has no table, the command has).
func checkVocabulary(d Decision) error {
	if len(d.Words) == 0 {
		return nil
	}
	voc := VocabularyOf(d.Rule)
	for _, w := range d.Words {
		i := sort.SearchStrings(voc, w)
		if i >= len(voc) || voc[i] != w {
			return fmt.Errorf("adopt_rules %s (%s): word %q is not in the vocabulary of %s (%s)", d.ID, d.File, w, d.Rule, strings.Join(voc, ", "))
		}
	}
	return nil
}

// scanDecision returns the claims of the decision (every position that the rule produces, in the scope) and the candidates they come from.
func scanDecision(api *API, d Decision) ([]Claim, map[string]Item) {
	var claims []Claim
	items := map[string]Item{}
	add := func(it Item, pos string) {
		cat := ""
		if d.Kind == "sink" {
			cat = d.Category
		}
		claims = append(claims, Claim{Decision: d.ID, Symbol: it.Symbol, Position: pos, Category: cat})
		items[it.Kind+"\x00"+it.Symbol] = it
	}
	if d.Rule == model.RuleSinkArgType {
		syms := make([]string, 0, len(api.Params))
		for s := range api.Params {
			syms = append(syms, s)
		}
		sort.Strings(syms)
		for _, s := range syms {
			it := Item{Symbol: s, Kind: KindSink, PosTypes: map[string]string{}}
			var pos []string
			for _, p := range api.Params[s] {
				for _, t := range d.ArgTypes {
					if p.Type == t {
						pos = append(pos, p.Pos)
						it.PosTypes[p.Pos] = p.Type
					}
				}
			}
			if len(pos) == 0 || !inScope(d.Scope, it) {
				continue
			}
			it.Positions = pos
			for _, p := range pos {
				add(it, p)
			}
		}
		return claims, items
	}
	for _, it := range api.Items {
		if it.Kind != d.Kind || !inScope(d.Scope, it) {
			continue
		}
		for _, pos := range it.Positions {
			for _, t := range it.Tags[pos] {
				if t.Rule == d.Rule && wordsMatch(d, t) {
					add(it, pos)
					break
				}
			}
		}
	}
	return claims, items
}

// ---- what a hand row, rejected: and deferred: already decide ----

func parsePos(pos string) (arg int, field string, ok bool) {
	if !strings.HasPrefix(pos, "arg") {
		return 0, "", false
	}
	rest := pos[3:]
	if i := strings.Index(rest, "."); i >= 0 {
		rest, field = rest[:i], pos[3+i+1:]
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0, "", false
	}
	return n, field, true
}

// sinkRowCovers says whether a hand row of sinks has the position: its `args`, or a part (arg, field) of it; a row that names no position is
// dangerous at every one.
func sinkRowCovers(s model.Sink, pos string) bool {
	n, field, ok := parsePos(pos)
	if !ok {
		return false
	}
	parts := s.PartList()
	if len(s.Args) == 0 && len(parts) == 0 && !s.Recv && len(s.Named) == 0 {
		return true
	}
	for _, a := range s.Args {
		if a == n {
			return true
		}
	}
	if field != "" {
		for _, p := range parts {
			if p.Arg != nil && *p.Arg == n && (p.Field == field || strings.HasPrefix(field, p.Field+".")) {
				return true
			}
		}
	}
	return false
}

// rowPositions are the positions that a hand row of sinks names (for the count of the rows that no decision covers).
func rowPositions(s model.Sink) []string {
	var out []string
	for _, a := range s.Args {
		out = append(out, "arg"+strconv.Itoa(a))
	}
	for _, p := range s.PartList() {
		if p.Arg != nil {
			out = append(out, "arg"+strconv.Itoa(*p.Arg)+"."+p.Field)
		}
	}
	return out
}

// rejectedAt says whether `rejected:` decides the place (the same reading as Match); deferredAt: the same for `deferred:`.
func rejectedAt(f *Facts, it Item, pos string) bool {
	for _, r := range f.Rejected {
		if r.Kind != it.Kind {
			continue
		}
		switch {
		case r.Type != "":
			if it.PosTypes[pos] == r.Type {
				return true
			}
		case r.Symbol == it.Symbol && (r.Position == "" || r.Position == pos):
			return true
		}
	}
	return false
}

func deferredAt(f *Facts, it Item, pos string) bool {
	for _, d := range f.Deferred {
		if d.Kind == it.Kind && d.Symbol == it.Symbol && (d.Position == "" || d.Position == pos) {
			return true
		}
	}
	return false
}

func handAt(f *Facts, hand map[string][]model.Sink, it Item, pos string) bool {
	if it.Kind == KindSink {
		for _, s := range hand[it.Symbol] {
			if sinkRowCovers(s, pos) {
				return true
			}
		}
		return false
	}
	return f.rowFor(it) != ""
}

// ---- Expand ----

// previousSymbols are the symbols of the rows of the decisions in a file of generated rows that exists (the output of the last run).
func previousSymbols(path, major string) map[string]bool {
	out := map[string]bool{}
	fh, err := os.Open(path)
	if err != nil {
		return out
	}
	defer fh.Close()
	rs, err := model.Parse(fh, path)
	if err != nil {
		return out
	}
	conc := func(s string) string {
		if major != "" && model.HasMajorWildcard(s) {
			return model.ConcreteMajor(s, major)
		}
		return s
	}
	for _, s := range rs.Sinks {
		out[conc(s.Symbol)] = true
	}
	for _, s := range rs.Sources {
		out[conc(s.Symbol)] = true
		out[conc(s.Field)] = true
	}
	return out
}

// Expand computes the rows of the decisions of the package. prev are the symbols that the file of generated rows had before (for the list of
// the new ones).
func Expand(api *API, f *Facts, prev map[string]bool) *ExpandResult {
	res := &ExpandResult{}
	hand := map[string][]model.Sink{}
	for _, s := range f.SinkRows {
		hand[s.Symbol] = append(hand[s.Symbol], s)
	}
	type perDecision struct {
		claimed, byHand, byReject map[string]bool
	}
	pd := map[string]*perDecision{}
	var remaining []Claim
	claimedAt := map[string]map[string]bool{} // symbol -> positions that any decision claims (before the suppression)
	kindOf := map[string]Decision{}
	for _, d := range f.Adopt {
		kindOf[d.ID] = d
		if err := checkVocabulary(d); err != nil {
			res.VocabularyError = err
			return res
		}
		claims, items := scanDecision(api, d)
		p := &perDecision{map[string]bool{}, map[string]bool{}, map[string]bool{}}
		pd[d.ID] = p
		for _, c := range claims {
			it := items[d.Kind+"\x00"+c.Symbol]
			p.claimed[c.Symbol] = true
			if claimedAt[c.Symbol] == nil {
				claimedAt[c.Symbol] = map[string]bool{}
			}
			claimedAt[c.Symbol][c.Position] = true
			switch {
			case handAt(f, hand, it, c.Position):
				p.byHand[c.Symbol] = true
			case rejectedAt(f, it, c.Position) || deferredAt(f, it, c.Position):
				p.byReject[c.Symbol] = true
			default:
				remaining = append(remaining, c)
			}
		}
	}
	merged, warns, err := ResolveClaims(remaining)
	res.Warnings = warns
	res.ConflictErr = err
	res.Rows = buildRows(merged, kindOf)

	for _, d := range f.Adopt {
		p := pd[d.ID]
		c := DecisionCount{ID: d.ID, Covers: *d.Covers, Claimed: keys(p.claimed), ByHand: keys(p.byHand), ByReject: keys(p.byReject)}
		for _, s := range c.Claimed {
			if !prev[s] && !p.byHand[s] {
				c.New = append(c.New, s)
			}
		}
		var mine []GenRow
		for _, r := range res.Rows {
			for _, id := range r.Decisions {
				if id == d.ID {
					mine = append(mine, r)
					c.Rows++
					break
				}
			}
		}
		c.RepsMissing, c.RepsAdvise = checkRepresentatives(d, mine)
		res.Counts = append(res.Counts, c)
	}

	// the hand rows that no decision covers
	covered := func(sym string, positions []string) bool {
		cl := claimedAt[sym]
		if cl == nil {
			return false
		}
		if len(positions) == 0 {
			return true
		}
		for _, p := range positions {
			base := p
			if i := strings.Index(p, "."); i >= 0 {
				base = p[:i]
			}
			if cl[p] || cl[base] {
				continue
			}
			found := false
			for q := range cl {
				if strings.HasPrefix(q, base+".") {
					found = true
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	var notCovered []string
	for _, s := range f.SinkRows {
		res.HandTotal++
		pos := rowPositions(s)
		if covered(s.Symbol, pos) {
			res.HandCovered++
		} else {
			notCovered = append(notCovered, fmt.Sprintf("sink %s %s", s.Symbol, strings.Join(pos, ",")))
		}
	}
	for _, s := range f.SourceRows {
		sym := s.Symbol
		if s.Field != "" {
			sym = s.Field
		}
		if sym == "" {
			continue // a param_type row names a type, not a symbol
		}
		res.HandTotal++
		if covered(sym, nil) {
			res.HandCovered++
		} else {
			notCovered = append(notCovered, "source "+sym)
		}
	}
	sort.Strings(notCovered)
	res.HandNotCovered = notCovered
	return res
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// buildRows turns the merged positions into rows: one row per (symbol, kind, category, decisions); a sink row has the positions as `args` and `parts`.
func buildRows(merged []Merged, dec map[string]Decision) []GenRow {
	type key struct{ sym, kind, cat, ds string }
	idx := map[key]*GenRow{}
	var order []key
	for _, m := range merged {
		d := dec[m.Decisions[0]]
		k := key{m.Symbol, d.Kind, m.Category, strings.Join(m.Decisions, ",")}
		r := idx[k]
		if r == nil {
			r = &GenRow{Kind: d.Kind, Symbol: m.Symbol, Category: m.Category, Decisions: m.Decisions}
			if d.Kind == "source" {
				r.Field = m.Position == "field"
				labels := map[string]bool{}
				for _, id := range m.Decisions {
					for _, l := range dec[id].Labels {
						labels[l] = true
					}
				}
				r.Labels = keys(labels)
			}
			idx[k] = r
			order = append(order, k)
		}
		if d.Kind != "sink" {
			continue
		}
		if n, field, ok := parsePos(m.Position); ok {
			if field == "" {
				r.Args = append(r.Args, n)
			} else {
				r.Parts = append(r.Parts, GenPart{n, field})
			}
		}
	}
	var rows []GenRow
	for _, k := range order {
		r := idx[k]
		sort.Ints(r.Args)
		sort.SliceStable(r.Parts, func(i, j int) bool {
			if r.Parts[i].Arg != r.Parts[j].Arg {
				return r.Parts[i].Arg < r.Parts[j].Arg
			}
			return r.Parts[i].Field < r.Parts[j].Field
		})
		rows = append(rows, *r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Symbol != b.Symbol {
			return a.Symbol < b.Symbol
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		return strings.Join(a.Decisions, ",") < strings.Join(b.Decisions, ",")
	})
	return rows
}

// shape is the combination of positions of a row (`arg1,arg2`, `arg0.Object`, `field`, `result`): what the representatives have to cover.
func (r GenRow) shape() string {
	if r.Kind == "source" {
		if r.Field {
			return "field"
		}
		return "result"
	}
	var p []string
	for _, a := range r.Args {
		p = append(p, "arg"+strconv.Itoa(a))
	}
	for _, x := range r.Parts {
		p = append(p, "arg"+strconv.Itoa(x.Arg)+"."+x.Field)
	}
	return strings.Join(p, ",")
}

// checkRepresentatives: every representative of the decision is a generated row of it; and the combinations of positions (the 3 with the most rows,
// at most) each have one. missing are the representatives that are not rows of the decision; advise is the first symbol (in dictionary order) of each
// combination that has no representative.
func checkRepresentatives(d Decision, rows []GenRow) (missing, advise []string) {
	byShape := map[string][]string{}
	isRow := map[string]bool{}
	for _, r := range rows {
		isRow[r.Symbol] = true
		byShape[r.shape()] = append(byShape[r.shape()], r.Symbol)
	}
	reps := map[string]bool{}
	for _, s := range d.Representatives {
		reps[s] = true
		if !isRow[s] {
			missing = append(missing, s)
		}
	}
	type sh struct {
		name string
		n    int
	}
	var shapes []sh
	for k, v := range byShape {
		shapes = append(shapes, sh{k, len(v)})
	}
	sort.Slice(shapes, func(i, j int) bool {
		if shapes[i].n != shapes[j].n {
			return shapes[i].n > shapes[j].n
		}
		return shapes[i].name < shapes[j].name
	})
	if len(shapes) > 3 {
		shapes = shapes[:3]
	}
	for _, s := range shapes {
		has := false
		for _, sym := range byShape[s.name] {
			has = has || reps[sym]
		}
		if !has {
			syms := append([]string{}, byShape[s.name]...)
			sort.Strings(syms)
			advise = append(advise, syms[0]+" ("+s.name+")")
		}
	}
	sort.Strings(missing)
	sort.Strings(advise)
	return missing, advise
}

// ---- the file ----

// tableHash is the hash of the rule table and of its vocabulary: what `rules:` of the header records.
func tableHash() string {
	h := sha256.New()
	for _, r := range ruleTable {
		fmt.Fprintf(h, "%s|%s|%s\n", r.ID, r.Kind, r.Text)
	}
	for _, c := range categoryOrder {
		fmt.Fprintf(h, "%s:%s\n", c, strings.Join(sinkWords[c], ","))
	}
	for _, l := range [][]string{bytesSinkWords, resourceNouns, fileVerbs, sourceWords, notTextWords, sanitizerWords} {
		fmt.Fprintf(h, "%s\n", strings.Join(l, ","))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// decisionsHash is the hash of what of the decisions changes the rows (not covers, why, representatives, ...).
func decisionsHash(ds []Decision) string {
	type view struct {
		ID, Kind, Rule, Category string
		Words, ArgTypes, Labels  []string
		Scope                    model.AdoptScope
	}
	var vs []view
	for _, d := range ds {
		vs = append(vs, view{d.ID, d.Kind, d.Rule, d.Category, d.Words, d.ArgTypes, d.Labels, d.Scope})
	}
	b, _ := json.Marshal(vs)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

var wildPkgRe = regexp.MustCompile(`/v\*`)

// RenderGen writes the file of generated rows. The symbols are written with the package as the decision wrote it (`/v*` for the major).
func RenderGen(res *ExpandResult, f *Facts, lang, stem, pkg, input string) string {
	wild := map[string]string{}
	for _, d := range f.Adopt {
		wild[d.ID] = d.PackageWild
	}
	sym := func(r GenRow) string {
		if w := wild[r.Decisions[0]]; w != "" && w != pkg && wildPkgRe.MatchString(w) {
			return strings.ReplaceAll(r.Symbol, pkg, w)
		}
		return r.Symbol
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# The rows of the decisions of adopt_rules: in %s.yaml, expanded by `make model ARGS=\"-expand\"` (bench/model-author). Do not edit: change the\n", stem)
	b.WriteString("# decision or add a hand row (a hand row wins), then expand again. The same input gives the same bytes.\n")
	fmt.Fprintf(&b, "ruleset: rivus/%s/%s-adopt@1\n", lang, stem)
	fmt.Fprintf(&b, "generator: model-author@%s\n", GeneratorVersion)
	fmt.Fprintf(&b, "input: %s\n", input)
	fmt.Fprintf(&b, "rules: %q\n", tableHash())
	fmt.Fprintf(&b, "decisions: %q\n", decisionsHash(f.Adopt))
	decision := func(r GenRow) string {
		if len(r.Decisions) == 1 {
			return r.Decisions[0]
		}
		return "[" + strings.Join(r.Decisions, ", ") + "]"
	}
	var sinks, sources []GenRow
	for _, r := range res.Rows {
		if r.Kind == "sink" {
			sinks = append(sinks, r)
		} else {
			sources = append(sources, r)
		}
	}
	if len(sinks) > 0 {
		b.WriteString("sinks:\n")
	}
	for _, r := range sinks {
		fmt.Fprintf(&b, "  - symbol: %s\n", yamlScalar(sym(r)))
		if len(r.Args) > 0 {
			ns := make([]string, len(r.Args))
			for i, a := range r.Args {
				ns[i] = strconv.Itoa(a)
			}
			fmt.Fprintf(&b, "    args: [%s]\n", strings.Join(ns, ", "))
		}
		if len(r.Parts) > 0 {
			ps := make([]string, len(r.Parts))
			for i, p := range r.Parts {
				ps[i] = fmt.Sprintf("{arg: %d, field: %s}", p.Arg, p.Field)
			}
			fmt.Fprintf(&b, "    parts: [%s]\n", strings.Join(ps, ", "))
		}
		fmt.Fprintf(&b, "    category: %s\n    origin: generated\n    decision: %s\n", r.Category, decision(r))
	}
	if len(sources) > 0 {
		b.WriteString("sources:\n")
	}
	for _, r := range sources {
		if r.Field {
			fmt.Fprintf(&b, "  - kind: field\n    field: %s\n", yamlScalar(sym(r)))
		} else {
			fmt.Fprintf(&b, "  - kind: call_result\n    symbol: %s\n", yamlScalar(sym(r)))
		}
		fmt.Fprintf(&b, "    labels: [%s]\n    origin: generated\n    decision: %s\n", strings.Join(r.Labels, ", "), decision(r))
	}
	return b.String()
}

// ---- covers written back ----

var coversRe = regexp.MustCompile(`^(\s*covers:\s*)\d+`)

// writeCovers sets `covers:` of the decision (the entry that starts at line d.Line) in the model file, and nothing else.
func writeCovers(path string, d Decision, n int) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	if d.Line < 1 || d.Line > len(lines) {
		return fmt.Errorf("%s: the entry of %s is not at a known line", path, d.ID)
	}
	for i := d.Line - 1; i < len(lines); i++ {
		if i > d.Line-1 && strings.HasPrefix(strings.TrimLeft(lines[i], " "), "- id:") {
			break
		}
		if coversRe.MatchString(lines[i]) {
			lines[i] = coversRe.ReplaceAllString(lines[i], "${1}"+strconv.Itoa(n))
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}
	return fmt.Errorf("%s: no `covers:` line in the entry of %s", path, d.ID)
}

// ---- the command ----

// runExpand is the mode `-expand`: expand the decisions of the package into <out>/<stem>.adopt.gen.yaml.
func runExpand(out io.Writer, o *options, repo string, f *Facts, api *API, version string) error {
	if len(f.Adopt) == 0 {
		return &exitError{ExitUsage, fmt.Errorf("no adopt_rules: for %s in the model files (the hand file has the decisions)", o.pkg)}
	}
	hand := f.Adopt[0].File
	for _, d := range f.Adopt {
		if d.File != hand {
			return &exitError{ExitUsage, fmt.Errorf("the decisions of %s are in two files (%s and %s): a package has one hand file", o.pkg, hand, d.File)}
		}
	}
	stem := strings.TrimSuffix(filepath.Base(hand), ".yaml")
	outDir := o.out
	if outDir == "" {
		outDir = filepath.Join(repo, "internal", "model", "builtin")
	}
	target := filepath.Join(outDir, stem+model.AdoptGenSuffix)
	res := Expand(api, f, previousSymbols(target, majorOf(o.pkg)))
	slug := slugRe.ReplaceAllString(o.pkg, "_")

	fmt.Fprintf(out, "model-author expand %s %s\n  decisions: %d in %s\n", o.pkg, version, len(f.Adopt), hand)
	if res.VocabularyError != nil {
		return &exitError{ExitUsage, res.VocabularyError}
	}
	var work strings.Builder
	exit70 := 0
	for _, c := range res.Counts {
		fmt.Fprintf(out, "  %s: claims %d symbol(s) (covers: %d); %d with a hand row, %d decided by rejected:/deferred:; %d generated row(s)\n",
			c.ID, len(c.Claimed), c.Covers, len(c.ByHand), len(c.ByReject), c.Rows)
		fmt.Fprintf(&work, "== %s: claims %d, covers %d\nclaimed:\n  %s\nsuppressed by a hand row:\n  %s\ndecided by rejected: or deferred:\n  %s\nnew (not in the previous file, no hand row):\n  %s\n",
			c.ID, len(c.Claimed), c.Covers, strings.Join(c.Claimed, "\n  "), strings.Join(c.ByHand, "\n  "), strings.Join(c.ByReject, "\n  "), strings.Join(c.New, "\n  "))
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(out, "  warning: %s\n", w)
	}
	fmt.Fprintf(out, "  hand rows of the package: %d of %d are covered by a decision, %d are not\n", res.HandCovered, res.HandTotal, len(res.HandNotCovered))
	fmt.Fprintf(&work, "== hand rows that no decision covers (%d of %d)\n  %s\n", len(res.HandNotCovered), res.HandTotal, strings.Join(res.HandNotCovered, "\n  "))
	wf := filepath.Join(o.work, slug+"-expansion.txt")
	if err := os.WriteFile(wf, []byte(work.String()), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "  the lists (claimed, suppressed, new, hand rows not covered): %s\n", wf)

	if res.ConflictErr != nil {
		fmt.Fprintln(out, res.ConflictErr.Error())
		return &exitError{ExitUndecided, fmt.Errorf("conflict between decisions (exit %d): the file is not written", ExitUndecided)}
	}
	// covers
	byID := map[string]Decision{}
	for _, d := range f.Adopt {
		byID[d.ID] = d
	}
	for _, c := range res.Counts {
		n := len(c.Claimed)
		switch {
		case n > c.Covers:
			if o.writeCovers {
				continue
			}
			shown := c.New
			if len(shown) > 20 {
				shown = append(append([]string{}, shown[:20]...), fmt.Sprintf("... and %d more (see %s)", len(c.New)-20, wf))
			}
			fmt.Fprintf(out, "  %s claims %d symbol(s), covers: %d: the new ones (a human decides: raise covers with -write-covers, or narrow the decision, or reject some):\n    %s\n", c.ID, n, c.Covers, strings.Join(shown, "\n    "))
			exit70++
		case n < c.Covers:
			fmt.Fprintf(out, "  warning: %s claims %d symbol(s), covers: %d: some of the API it covered is gone (or a rule no longer fires); update covers: (-write-covers)\n", c.ID, n, c.Covers)
		}
	}
	if exit70 > 0 {
		return &exitError{ExitUndecided, fmt.Errorf("%d decision(s) claim more than covers: (exit %d): the file is not written", exit70, ExitUndecided)}
	}
	if o.writeCovers {
		for _, c := range res.Counts {
			if n := len(c.Claimed); n != c.Covers {
				if err := writeCovers(hand, byID[c.ID], n); err != nil {
					return err
				}
				fmt.Fprintf(out, "  covers of %s: %d -> %d written to %s\n", c.ID, c.Covers, n, hand)
			}
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	text := RenderGen(res, f, o.lang, stem, o.pkg, o.pkg+"@"+version)
	old, _ := os.ReadFile(target)
	if string(old) == text {
		fmt.Fprintf(out, "  %s: unchanged (%d row(s))\n", target, len(res.Rows))
	} else {
		if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "  %s: written (%d row(s))\n", target, len(res.Rows))
	}
	bad := false
	for _, c := range res.Counts {
		for _, s := range c.RepsMissing {
			fmt.Fprintf(out, "  representative of %s is not a generated row of it: %s\n", c.ID, s)
			bad = true
		}
		for _, s := range c.RepsAdvise {
			fmt.Fprintf(out, "  warning: %s has no representative for a combination of positions: add one, for instance %s\n", c.ID, s)
		}
	}
	if bad {
		return &exitError{ExitUndecided, fmt.Errorf("a representative is not a generated row of its decision (exit %d)", ExitUndecided)}
	}
	return nil
}
