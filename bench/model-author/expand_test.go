package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
)

const fakePkg = "example.com/fake/v7"

// loadFake reads the synthetic package under testdata/adoptpkg and its hand file.
func loadFake(t *testing.T, handFile string) (*API, *Facts) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "testdata/adoptpkg/fake.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check(fakePkg, fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := LoadFacts([]string{handFile}, fakePkg)
	if err != nil {
		t.Fatal(err)
	}
	return Scan(pkg, false), facts
}

func fakeHand() string { return "testdata/adoptpkg/go-fake.yaml" }

func symbols(rows []GenRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Symbol+" "+r.shape()+" "+strings.Join(r.Decisions, "+"))
	}
	return out
}

func TestScanTagsPositionsWithRuleIDs(t *testing.T) {
	api := scanSynth(t)
	// the tags: every position of every candidate has a tag, and the tag is the id of a rule of the table
	ids := map[string]bool{}
	for _, id := range RuleIDs() {
		ids[id] = true
	}
	n := 0
	for _, it := range api.Items {
		if it.Kind == KindCalls {
			continue
		}
		for _, pos := range it.Positions {
			tags := it.Tags[pos]
			if len(tags) == 0 {
				t.Errorf("%s %s: no rule id", it.Symbol, pos)
			}
			for _, tg := range tags {
				n++
				if !ids[tg.Rule] || !strings.HasPrefix(tg.Rule, it.Kind+".") {
					t.Errorf("%s %s: tag %q is not a rule of kind %s", it.Symbol, pos, tg.Rule, it.Kind)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no tags")
	}
	find := func(sym, kind string) Item {
		for _, it := range api.Items {
			if it.Symbol == sym && it.Kind == kind {
				return it
			}
		}
		t.Fatalf("no candidate %s %s", kind, sym)
		return Item{}
	}
	for _, c := range []struct{ sym, kind, pos, rule, word string }{
		{"example.com/synth.Query", KindSink, "arg1", "sink.arg-word", "query"},
		{"example.com/synth.Open", KindSink, "arg0", "sink.arg-word", "path"},
		{"(*example.com/synth.Client).Bucket", KindSink, "arg0", "sink.name-of-resource", ""},
		{"(*example.com/synth.Client).Move", KindSink, "arg1.Object", "sink.struct-field", "object"},
		{"example.com/synth.Print", KindSink, "arg2", "sink.format-values", ""},
		{"example.com/synth.Body", KindSource, "result", "source.result-word", "body"},
		{"example.com/synth.Doc.Body", KindSource, "field", "source.field-word", "body"},
		{"example.com/synth.EscapeText", KindSanitizer, "result", "sanitizer.name", "escape"},
	} {
		it := find(c.sym, c.kind)
		tags := it.Tags[c.pos]
		if len(tags) != 1 || tags[0].Rule != c.rule {
			t.Errorf("%s %s: tags %v, want %s", c.sym, c.pos, tags, c.rule)
			continue
		}
		if c.word != "" && !(len(tags[0].Words) == 1 && tags[0].Words[0] == c.word) {
			t.Errorf("%s %s: words %v, want %s", c.sym, c.pos, tags[0].Words, c.word)
		}
	}
}

// -recall must not change: the tags are not in the JSON of an item.
func TestTagsAreNotInTheItemJSON(t *testing.T) {
	api := scanSynth(t)
	for _, it := range api.Items {
		if len(it.Tags) == 0 && it.Kind != KindCalls {
			t.Errorf("%s has no tags", it.Symbol)
		}
	}
	b, _ := jsonOf(api.Items)
	if strings.Contains(b, "Tags") || strings.Contains(b, "sink.arg-word") {
		t.Errorf("the tags are in the JSON of the items: %.200s", b)
	}
}

func TestVocabularyOf(t *testing.T) {
	if v := VocabularyOf("sink.arg-word"); len(v) < 30 {
		t.Errorf("sink.arg-word vocabulary too small: %v", v)
	}
	if VocabularyOf("sink.file-name") != nil {
		t.Error("sink.file-name has a fixed vocabulary")
	}
}

func TestExpandSyntheticPackage(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	res := Expand(api, facts, nil)
	if res.ConflictErr != nil || res.VocabularyError != nil {
		t.Fatal(res.ConflictErr, res.VocabularyError)
	}
	p := "(*example.com/fake/v7."
	want := []string{
		// OrgService.Edit has a hand row, Old is rejected: neither is generated; Client.Get and Top are not in the scope
		p + "IssueService).Create arg1,arg2 owner-repo-of-services",
		p + "IssueService).Move arg0,arg1 slug-of-services",
		p + "OrgService).Fetch arg1 owner-repo-of-services",
		p + "OrgService).Find arg1 slug-of-services",
		p + "OrgService).Get arg1,arg2 owner-repo-of-services",
		p + "OrgService).List arg1 owner-repo-of-services",
		"example.com/fake/v7.Doc.Body field text-of-doc",
	}
	if got := symbols(res.Rows); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	byID := map[string]DecisionCount{}
	for _, c := range res.Counts {
		byID[c.ID] = c
	}
	o := byID["owner-repo-of-services"]
	// claims Create, Edit, Fetch, Get, List, Old (Client.Get and Top are out of the scope): Edit has a hand row, Old is rejected
	if len(o.Claimed) != 6 || len(o.ByHand) != 1 || len(o.ByReject) != 1 || o.Rows != 4 {
		t.Errorf("owner-repo-of-services: %+v", o)
	}
	if s := byID["text-of-doc"]; len(s.Claimed) != 2 || len(s.ByHand) != 1 || s.Rows != 1 {
		t.Errorf("text-of-doc: %+v", s)
	}
	// the hand rows: Edit (covered), Client.Get (no decision), the Title field (covered)
	if res.HandTotal != 3 || res.HandCovered != 2 || len(res.HandNotCovered) != 1 || !strings.Contains(res.HandNotCovered[0], "Client).Get") {
		t.Errorf("hand rows: %d of %d, not covered %v", res.HandCovered, res.HandTotal, res.HandNotCovered)
	}
}

func TestExpandScopeRestrictions(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	base := facts.Adopt[0]
	count := func(sc model.AdoptScope) int {
		d := base
		d.Scope = sc
		claims, _ := scanDecision(api, d)
		return len(claims)
	}
	pkg := model.AdoptScope{Package: fakePkg}
	// no receiver key: also Client.Get and Top
	if n := count(pkg); n != 11 {
		t.Errorf("package only: %d claims", n)
	}
	sc := pkg
	sc.ReceiverSuffix = "Service"
	if n := count(sc); n != 9 {
		t.Errorf("receiver_suffix Service: %d", n)
	}
	sc = pkg
	sc.ReceiverPrefix = "Org"
	if n := count(sc); n != 7 {
		t.Errorf("receiver_prefix Org: %d", n)
	}
	sc = pkg
	sc.ReceiverTypes = []string{fakePkg + ".Client"}
	if n := count(sc); n != 1 {
		t.Errorf("receiver_types Client: %d", n)
	}
	sc = pkg
	sc.Functions = []string{"Top"}
	if n := count(sc); n != 1 {
		t.Errorf("functions Top: %d (a function has no receiver, and no receiver key is set)", n)
	}
	sc = pkg
	sc.Functions = []string{"Get", "List"}
	sc.ReceiverSuffix = "Service"
	if n := count(sc); n != 3 {
		t.Errorf("functions and suffix (AND): %d", n)
	}
	sc = pkg
	sc.ReceiverSuffix = "Service"
	sc.Functions = []string{"Top"}
	if n := count(sc); n != 0 {
		t.Errorf("a function with a receiver key is out: %d", n)
	}
}

func TestExpandWordsAreFromTheVocabulary(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	facts.Adopt[0].Words = []string{"owner", "nonsense"}
	res := Expand(api, facts, nil)
	if res.VocabularyError == nil || !strings.Contains(res.VocabularyError.Error(), `"nonsense"`) {
		t.Fatalf("got %v", res.VocabularyError)
	}
}

func TestArgTypeRule(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	var slug Decision
	for _, d := range facts.Adopt {
		if d.ID == "slug-of-services" {
			slug = d
		}
	}
	claims, _ := scanDecision(api, slug)
	var got []string
	for _, c := range claims {
		got = append(got, c.Symbol+" "+c.Position)
	}
	// a pointer is read through (`*Slug`), both arguments of Move; Client.Find is not a Service; the string `note` is not a Slug
	want := []string{
		"(*example.com/fake/v7.IssueService).Move arg0", "(*example.com/fake/v7.IssueService).Move arg1", "(*example.com/fake/v7.OrgService).Find arg1",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// the rule is not applied by the scan itself (no candidate of it)
	for _, it := range api.Items {
		for _, tags := range it.Tags {
			for _, tg := range tags {
				if tg.Rule == model.RuleSinkArgType {
					t.Errorf("%s: the scan applied sink.arg-type", it.Symbol)
				}
			}
		}
	}
}

func TestRejectedByTypeSuppressesArgType(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	facts.Rejected = append(facts.Rejected, RejectedEntry{Rejected: model.Rejected{Type: fakePkg + ".Slug", Kind: "sink", Why: "x"}})
	res := Expand(api, facts, nil)
	for _, r := range res.Rows {
		if strings.Contains(strings.Join(r.Decisions, ","), "slug-of-services") {
			t.Errorf("%s is generated though rejected by type", r.Symbol)
		}
	}
}

func TestExpandConflicts(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	other := facts.Adopt[0]
	other.ID = "another"
	other.Words = []string{"owner"}
	// the same category: merged, a warning, the row names both decisions (dictionary order)
	facts.Adopt = append(facts.Adopt, other)
	res := Expand(api, facts, nil)
	if res.ConflictErr != nil {
		t.Fatal(res.ConflictErr)
	}
	if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "another and owner-repo-of-services") {
		t.Errorf("warnings: %v", res.Warnings)
	}
	// Get: arg1 is claimed by both decisions (one row, both ids), arg2 by one of them (its own row)
	var got []string
	for _, r := range res.Rows {
		if strings.HasSuffix(r.Symbol, "OrgService).Get") {
			got = append(got, r.shape()+" "+strings.Join(r.Decisions, "+"))
		}
	}
	if strings.Join(got, "|") != "arg1 another+owner-repo-of-services|arg2 owner-repo-of-services" {
		t.Errorf("merged rows: %v", got)
	}
	// different categories: a conflict (exit 70 in the command)
	facts.Adopt[len(facts.Adopt)-1].Category = "ssrf"
	res = Expand(api, facts, nil)
	var ce *ConflictError
	if res.ConflictErr == nil || !errorsAs(res.ConflictErr, &ce) || len(ce.Conflicts) == 0 || !strings.Contains(res.ConflictErr.Error(), "ssrf, resource_name") && !strings.Contains(res.ConflictErr.Error(), "resource_name, ssrf") {
		t.Errorf("conflict: %v", res.ConflictErr)
	}
}

func TestExpansionIsDeterministicAndIdempotent(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	render := func() string {
		res := Expand(api, facts, nil)
		return RenderGen(res, facts, "go", "go-fake", fakePkg, fakePkg+"@v7.0.0")
	}
	a, b := render(), render()
	if a != b {
		t.Fatal("two runs differ")
	}
	// the order of the decisions in the file does not change the rows (it changes the hash of the decisions only)
	sw := *facts
	sw.Adopt = []Decision{facts.Adopt[2], facts.Adopt[0], facts.Adopt[1]}
	res := Expand(api, &sw, nil)
	c := RenderGen(res, &sw, "go", "go-fake", fakePkg, fakePkg+"@v7.0.0")
	strip := func(s string) string {
		var out []string
		for _, l := range strings.Split(s, "\n") {
			if !strings.HasPrefix(l, "decisions:") {
				out = append(out, l)
			}
		}
		return strings.Join(out, "\n")
	}
	if strip(a) != strip(c) {
		t.Errorf("the order of the decisions changes the rows:\n%s\n%s", a, c)
	}
	if !strings.Contains(a, "symbol: (*example.com/fake/v*.OrgService).Get\n    args: [1, 2]\n    category: resource_name\n    origin: generated\n    decision: owner-repo-of-services\n") {
		t.Errorf("a row is not as expected (the major is /v*):\n%s", a)
	}
	if !strings.Contains(a, "kind: field\n    field: example.com/fake/v*.Doc.Body\n    labels: [user_input]\n") {
		t.Errorf("the source row:\n%s", a)
	}
}

// the file loads together with its hand file (the loader and CheckAdoption of step 1) and its rows are the ones that were made.
func TestGeneratedFileLoadsWithTheHandFile(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	res := Expand(api, facts, nil)
	text := RenderGen(res, facts, "go", "go-fake", fakePkg, fakePkg+"@v7.0.0")
	dir := t.TempDir()
	gen := filepath.Join(dir, "go-fake.adopt.gen.yaml")
	if err := os.WriteFile(gen, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	var sets []*model.RuleSet
	for _, p := range []string{fakeHand(), gen} {
		fh, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		rs, err := model.Parse(fh, "builtin:"+filepath.Base(p))
		fh.Close()
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		sets = append(sets, rs)
	}
	if err := model.CheckAdoption(sets...); err != nil {
		t.Fatal(err)
	}
	if len(sets[1].Sinks) != 6 || len(sets[1].Sources) != 1 || sets[1].Generator == "" || sets[1].Rules != tableHash() {
		t.Errorf("sinks %d, sources %d, header %q %q", len(sets[1].Sinks), len(sets[1].Sources), sets[1].Generator, sets[1].Rules)
	}
}

func runFake(t *testing.T, mod func(*options), hand string) (string, int, string) {
	t.Helper()
	api, facts := loadFake(t, hand)
	dir := t.TempDir()
	o := &options{pkg: fakePkg, lang: "go", work: dir, out: dir, expand: true}
	if mod != nil {
		mod(o)
	}
	var buf bytes.Buffer
	err := runExpand(&buf, o, ".", facts, api, "v7.0.0")
	code := 0
	if ee, ok := err.(*exitError); ok {
		code = ee.code
	} else if err != nil {
		t.Fatal(err)
	}
	return buf.String(), code, o.out
}

func TestRunExpandWritesToOutAndIsIdempotent(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	dir := t.TempDir()
	o := &options{pkg: fakePkg, lang: "go", work: dir, out: dir, expand: true}
	var b1, b2 bytes.Buffer
	if err := runExpand(&b1, o, ".", facts, api, "v7.0.0"); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "go-fake.adopt.gen.yaml")
	first, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b1.String(), "written (7 row(s))") || !strings.Contains(b1.String(), "hand rows of the package: 2 of 3 are covered") {
		t.Errorf("report:\n%s", b1.String())
	}
	if err := runExpand(&b2, o, ".", facts, api, "v7.0.0"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(f)
	if !bytes.Equal(first, second) || !strings.Contains(b2.String(), "unchanged") {
		t.Errorf("not idempotent:\n%s", b2.String())
	}
	// with the file of the last run in the default place, it is not read as a hand row: the same hand file, the same rows
	if _, err := os.Stat("testdata/adoptpkg/go-fake.adopt.gen.yaml"); err == nil {
		t.Error("the test wrote into testdata")
	}
}

func copyHand(t *testing.T, edit func(string) string) string {
	t.Helper()
	b, err := os.ReadFile(fakeHand())
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "go-fake.yaml")
	if err := os.WriteFile(p, []byte(edit(string(b))), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCoversGrowthStopsWithExit70AndWritesNothing(t *testing.T) {
	hand := copyHand(t, func(s string) string { return strings.Replace(s, "covers: 6", "covers: 4", 1) })
	out, code, dir := runFake(t, nil, hand)
	if code != ExitUndecided {
		t.Fatalf("exit %d, want 70:\n%s", code, out)
	}
	if !strings.Contains(out, "claims 6 symbol(s), covers: 4") || !strings.Contains(out, "OrgService).Fetch") {
		t.Errorf("the new symbols are not listed:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "go-fake.adopt.gen.yaml")); err == nil {
		t.Error("the file was written though the command stopped")
	}
}

func TestCoversShrinkIsAWarning(t *testing.T) {
	hand := copyHand(t, func(s string) string { return strings.Replace(s, "covers: 6", "covers: 9", 1) })
	out, code, _ := runFake(t, nil, hand)
	if code != 0 || !strings.Contains(out, "warning: owner-repo-of-services claims 6 symbol(s), covers: 9") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

func TestWriteCoversOnlyWithTheFlag(t *testing.T) {
	hand := copyHand(t, func(s string) string { return strings.Replace(s, "covers: 6", "covers: 4", 1) })
	before, _ := os.ReadFile(hand)
	runFake(t, nil, hand)
	if after, _ := os.ReadFile(hand); !bytes.Equal(before, after) {
		t.Fatal("covers was written without -write-covers")
	}
	out, code, _ := runFake(t, func(o *options) { o.writeCovers = true }, hand)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	after, _ := os.ReadFile(hand)
	if !strings.Contains(string(after), "covers: 6") || strings.Contains(string(after), "covers: 4") {
		t.Errorf("covers not written:\n%s", after)
	}
	if diff := strings.Count(string(after), "\n") - strings.Count(string(before), "\n"); diff != 0 {
		t.Errorf("the file changed in more than a number (%d lines)", diff)
	}
	if !strings.Contains(out, "covers of owner-repo-of-services: 4 -> 6") {
		t.Errorf("report:\n%s", out)
	}
	// the other decisions are left as they are
	if !strings.Contains(string(after), "covers: 2") {
		t.Errorf("other covers changed:\n%s", after)
	}
}

func TestConflictStopsWithExit70(t *testing.T) {
	hand := copyHand(t, func(s string) string {
		return strings.Replace(s, "  - id: slug-of-services", `  - id: ssrf-owner
    kind: sink
    rule: sink.arg-word
    words: [owner]
    category: ssrf
    scope:
      package: example.com/fake/v*
      receiver_types: [example.com/fake/v*.OrgService]
    why: "x"
    covers: 3
  - id: slug-of-services`, 1)
	})
	out, code, dir := runFake(t, nil, hand)
	if code != ExitUndecided || !strings.Contains(out, "claim categories") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "go-fake.adopt.gen.yaml")); err == nil {
		t.Error("the file was written though there is a conflict")
	}
}

func TestRepresentatives(t *testing.T) {
	// a representative that is not a generated row of the decision stops (70) after the file is written
	hand := copyHand(t, func(s string) string {
		return strings.Replace(s, "(*example.com/fake/v*.OrgService).List", "(*example.com/fake/v*.OrgService).Edit", 1)
	})
	out, code, _ := runFake(t, nil, hand)
	if code != ExitUndecided || !strings.Contains(out, "representative of owner-repo-of-services is not a generated row of it: (*example.com/fake/v7.OrgService).Edit") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	// a combination that has no representative is advised; with all of them named there is nothing to say
	out, code, _ = runFake(t, nil, fakeHand())
	if code != 0 || strings.Contains(out, "no representative") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	hand = copyHand(t, func(s string) string {
		return strings.Replace(strings.Replace(s, "      - (*example.com/fake/v*.OrgService).Find\n", "", 1), "    representatives: [example.com/fake/v*.Doc.Body]\n", "", 1)
	})
	out, code, _ = runFake(t, nil, hand)
	if code != 0 || !strings.Contains(out, "slug-of-services has no representative for a combination of positions: add one, for instance (*example.com/fake/v7.OrgService).Find (arg1)") ||
		!strings.Contains(out, "text-of-doc has no representative") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

func TestRepresentativesPerCombination(t *testing.T) {
	api, facts := loadFake(t, fakeHand())
	res := Expand(api, facts, nil)
	var c DecisionCount
	for _, x := range res.Counts {
		if x.ID == "owner-repo-of-services" {
			c = x
		}
	}
	// Get (arg1,arg2) and List (arg1) are named; Create (arg1,arg2) and Fetch (arg1) share the combinations
	if len(c.RepsMissing) != 0 || len(c.RepsAdvise) != 0 {
		t.Errorf("%+v", c)
	}
}

func TestNoDecisionsIsUsageError(t *testing.T) {
	hand := copyHand(t, func(s string) string {
		i := strings.Index(s, "adopt_rules:")
		j := strings.Index(s, "sinks:")
		return s[:i] + s[j:]
	})
	out, code, _ := runFake(t, nil, hand)
	_ = out
	if code != ExitUsage {
		t.Errorf("exit %d", code)
	}
}

func TestExpandFlagsAreChecked(t *testing.T) {
	var out, errb bytes.Buffer
	if c := realMain([]string{"-pkg", "x", "-out", "d"}, &out, &errb); c != ExitUsage || !strings.Contains(errb.String(), "for -expand") {
		t.Errorf("-out without -expand: %d %s", c, errb.String())
	}
	errb.Reset()
	if c := realMain([]string{"-pkg", "dio", "-lang", "dart", "-expand"}, &out, &errb); c != ExitUsage || !strings.Contains(errb.String(), "Go only") {
		t.Errorf("dart: %d %s", c, errb.String())
	}
}

func jsonOf(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func errorsAs(err error, target **ConflictError) bool { return errors.As(err, target) }
