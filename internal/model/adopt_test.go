package model

import (
	"os"
	"strings"
	"testing"
)

const adoptHead = "ruleset: t@1\nadopt_rules:\n"

const adoptOK = `  - id: owner-repo
    kind: sink
    rule: sink.arg-word
    words: [owner, repo]
    category: resource_name
    scope: {package: example.com/x/v*, receiver_suffix: Service}
    why: "in the path"
    covers: 3
`

func parseAdopt(body string) (*RuleSet, error) {
	return Parse(strings.NewReader(adoptHead+body), "t.yaml")
}

func TestAdoptRulesAreReadAndNotUsed(t *testing.T) {
	rs, err := parseAdopt(adoptOK + `    representatives: ["(*example.com/x/v*.RepoService).Get"]
    diff_checked: {version: v1.2.3, count: 3}
    extra_ok: "same shape"
    extra_covers: 1
`)
	if err != nil {
		t.Fatal(err)
	}
	a := rs.AdoptRules[0]
	if a.ID != "owner-repo" || *a.Covers != 3 || a.Scope.ReceiverSuffix != "Service" || a.DiffChecked.Count != 3 || a.Line != 3 {
		t.Fatalf("%+v", a)
	}
	if r := Merge(rs); len(r.Symbols()) != 0 {
		t.Fatalf("a decision became a rule: %v", r.Symbols())
	}
}

func TestAdoptRuleArgType(t *testing.T) {
	_, err := parseAdopt(`  - id: slug
    kind: sink
    rule: sink.arg-type
    arg_types: ["package:github/src/common/model/repos.dart#RepositorySlug"]
    category: resource_name
    scope: {package: "package:github", receiver_suffix: Service}
    why: "in the path"
    covers: 10
`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAdoptRulesAreChecked(t *testing.T) {
	without := func(key string) string {
		var out []string
		for _, l := range strings.Split(adoptOK, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "- ")), key+":") {
				out = append(out, l)
			}
		}
		return strings.Join(out, "\n")
	}
	for name, c := range map[string]struct{ body, want string }{
		"no why":                      {without("why"), "why is required"},
		"blank why":                   {strings.Replace(adoptOK, `"in the path"`, `"  "`, 1), "why is required"},
		"no covers":                   {without("covers"), "covers is required"},
		"zero covers":                 {strings.Replace(adoptOK, "covers: 3", "covers: 0", 1), "covers is required"},
		"no id":                       {strings.Replace(adoptOK, "id: owner-repo", `id: ""`, 1), "id is required"},
		"unknown rule":                {strings.Replace(adoptOK, "sink.arg-word", "sink.arg-wrod", 1), "unknown rule"},
		"rule of the other kind":      {strings.Replace(adoptOK, "kind: sink", "kind: source", 1), "is not a rule of kind"},
		"bad kind":                    {strings.Replace(adoptOK, "kind: sink", "kind: calls", 1), "kind is required"},
		"no words":                    {without("words"), "words is required"},
		"two words":                   {strings.Replace(adoptOK, "[owner, repo]", "[owner, owner]", 1), "listed twice"},
		"no category":                 {without("category"), "category is required"},
		"unknown key":                 {adoptOK + "    colour: red\n", "colour"},
		"glob in package":             {strings.Replace(adoptOK, "example.com/x/v*", "example.com/*/github", 1), "pattern over symbols"},
		"glob in functions list":      {strings.Replace(adoptOK, "receiver_suffix: Service", "functions: [\"a.Get*\"]", 1), "pattern over symbols"},
		"no package":                  {strings.Replace(adoptOK, "package: example.com/x/v*, ", "", 1), "package is required"},
		"suffix and prefix":           {strings.Replace(adoptOK, "receiver_suffix: Service", "receiver_suffix: Service, receiver_prefix: Repo", 1), "one of them"},
		"glob in suffix":              {strings.Replace(adoptOK, "receiver_suffix: Service", `receiver_suffix: "*Service"`, 1), "plain letters"},
		"duplicate ids":               {adoptOK + adoptOK, "used twice"},
		"extra without count":         {adoptOK + "    extra_ok: why\n", "go together"},
		"diff without count":          {adoptOK + "    diff_checked: {version: v1}\n", "diff_checked needs"},
		"labels on a sink":            {adoptOK + "    labels: [user_input]\n", "labels is for a source"},
		"arg_types on the wrong rule": {adoptOK + "    arg_types: [a.B]\n", "arg_types is for"},
		"arg-type without types": {`  - id: s
    kind: sink
    rule: sink.arg-type
    category: resource_name
    scope: {package: p, receiver_suffix: Service}
    why: x
    covers: 1
`, "arg_types is required"},
		"arg-type with a whole package": {`  - id: s
    kind: sink
    rule: sink.arg-type
    arg_types: [p.T]
    category: resource_name
    scope: {package: p}
    why: x
    covers: 1
`, "narrower than a package"},
		"source without labels": {`  - id: s
    kind: source
    rule: source.field-word
    words: [body]
    scope: {package: p}
    why: x
    covers: 1
`, "labels is required"},
	} {
		_, err := parseAdopt(c.body)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", name, err, c.want)
		}
	}
}

func TestAdoptErrorNamesTheEntry(t *testing.T) {
	_, err := parseAdopt(adoptOK + strings.Replace(strings.Replace(adoptOK, "id: owner-repo", "id: second", 1), "covers: 3", "covers: 0", 1))
	if err == nil || !strings.Contains(err.Error(), "(second)") || !strings.Contains(err.Error(), "line 11") {
		t.Fatalf("%v", err)
	}
}

func TestAdoptRuleIDsAreOfTheirKind(t *testing.T) {
	for _, id := range AdoptRuleIDs {
		if !strings.HasPrefix(id, "sink.") && !strings.HasPrefix(id, "source.") {
			t.Errorf("%s", id)
		}
	}
}

func TestExtraWarning(t *testing.T) {
	two := 2
	a := AdoptRule{ID: "x", ExtraOK: "r", ExtraCovers: &two}
	if a.ExtraWarning(2) != "" || a.ExtraWarning(1) != "" {
		t.Error("warned without growth")
	}
	if w := a.ExtraWarning(3); !strings.Contains(w, "3 EXTRA") || !strings.Contains(w, "covered 2") {
		t.Errorf("%q", w)
	}
	if (AdoptRule{}).ExtraWarning(5) != "" {
		t.Error("warned without extra_ok")
	}
}

// ---- generated rows ----

func TestGeneratedRowNeedsADecision(t *testing.T) {
	_, err := Parse(strings.NewReader("ruleset: t@1\nsinks:\n  - symbol: a.B\n    origin: generated\n"), "go-x.adopt.gen.yaml")
	if err == nil || !strings.Contains(err.Error(), "needs decision") {
		t.Fatalf("%v", err)
	}
	_, err = Parse(strings.NewReader("ruleset: t@1\nsinks:\n  - symbol: a.B\n    decision: d\n"), "go-x.yaml")
	if err == nil || !strings.Contains(err.Error(), "decision is for the rows of origin generated") {
		t.Fatalf("%v", err)
	}
	// the default origin of a file is human, and builtin stays for the embedded files
	rs, err := Parse(strings.NewReader("ruleset: t@1\nsinks:\n  - symbol: a.B\n"), "go-x.yaml")
	if err != nil || rs.Sinks[0].Origin != OriginHuman {
		t.Fatalf("%v %+v", err, rs)
	}
}

func loadAdoptPair(t *testing.T, edit func(hand, gen string) (string, string)) (*RuleSet, *RuleSet, error) {
	t.Helper()
	h, _ := os.ReadFile("testdata/adopt/go-fake.yaml")
	g, _ := os.ReadFile("testdata/adopt/go-fake.adopt.gen.yaml")
	hs, gs := string(h), string(g)
	if edit != nil {
		hs, gs = edit(hs, gs)
	}
	hand, err := Parse(strings.NewReader(hs), "builtin:go-fake.yaml")
	if err != nil {
		return nil, nil, err
	}
	gen, err := Parse(strings.NewReader(gs), "builtin:go-fake.adopt.gen.yaml")
	if err != nil {
		return nil, nil, err
	}
	return hand, gen, CheckAdoption(hand, gen)
}

func TestAdoptGenFileLoadsWithItsHandFile(t *testing.T) {
	hand, gen, err := loadAdoptPair(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := Merge(hand, gen)
	if len(r.Sinks["(*example.com/fake/v*.OrgService).Get"]) == 0 || len(r.Sinks["(*example.com/fake/v*.Client).Get"]) == 0 {
		t.Fatalf("%v", r.Symbols())
	}
	if gen.Sinks[0].Origin != OriginGenerated || len(gen.Sinks[0].Decision) != 1 || gen.Sinks[0].Decision[0] != "owner-repo-of-services" {
		t.Fatalf("%+v", gen.Sinks[0])
	}
	if !IsAdoptGen("builtin:go-fake.adopt.gen.yaml") || IsAdoptGen("go-fake.gen.yaml") || IsAdoptGen("go-fake.yaml") {
		t.Error("IsAdoptGen")
	}
	// the analysis loads the rows of both
	if _, n := Merge(hand, gen), len(gen.Sinks); n != 2 {
		t.Fatal(n)
	}
}

func TestAdoptGenFileIsChecked(t *testing.T) {
	for name, c := range map[string]struct {
		edit func(hand, gen string) (string, string)
		want string
	}{
		"decision missing in the hand file": {func(h, g string) (string, string) {
			return h, strings.Replace(g, "decision: owner-repo-of-services", "decision: nobody", 1)
		}, `decision "nobody" is not in adopt_rules`},
		"no adopt_rules in the hand file": {func(h, g string) (string, string) {
			return strings.Replace(h, "adopt_rules:", "xadopt_rules:", 1), g
		}, "not found"},
		"a hand row of origin generated": {func(h, g string) (string, string) {
			return strings.Replace(h, "category: resource_name\nadopt", "category: resource_name\n    origin: generated\n    decision: owner-repo-of-services\nadopt", 1), g
		}, "named *.adopt.gen.yaml"},
		"a human row in the gen file": {func(h, g string) (string, string) {
			return h, strings.Replace(g, "origin: generated\n    decision: owner-repo-of-services", "origin: human", 1)
		}, "is origin generated"},
		"a source row for a sink decision": {func(h, g string) (string, string) {
			return h, g + "sources:\n  - symbol: a.B\n    labels: [user_input]\n    origin: generated\n    decision: owner-repo-of-services\n"
		}, "is a source, decision"},
		"a propagator in the gen file": {func(h, g string) (string, string) {
			return h, g + "propagators:\n  - symbol: a.C\n    flows: [{from: arg0, to: result}]\n"
		}, "sinks and sources only"},
		"decisions in the gen file": {func(h, g string) (string, string) {
			return h, "adopt_rules:\n" + strings.SplitN(h, "adopt_rules:\n", 2)[1] + g
		}, "not in the file of the generated rows"},
	} {
		_, _, err := loadAdoptPair(t, c.edit)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", name, err, c.want)
		}
	}
}

func TestDecisionList(t *testing.T) {
	two := func(l string) func(h, g string) (string, string) {
		return func(h, g string) (string, string) {
			h = strings.Replace(h, "adopt_rules:\n", "adopt_rules:\n  - id: other\n    kind: sink\n    rule: sink.file-name\n    category: path_traversal\n    scope: {package: example.com/fake/v*, receiver_suffix: Service}\n    why: x\n    covers: 1\n", 1)
			return h, strings.Replace(g, "decision: owner-repo-of-services\n  - symbol: (*example.com/fake/v*.OrgService).List", "decision: "+l+"\n  - symbol: (*example.com/fake/v*.OrgService).List", 1)
		}
	}
	_, gen, err := loadAdoptPair(t, two("[other, owner-repo-of-services]"))
	if err != nil || len(gen.Sinks[0].Decision) != 2 {
		t.Fatalf("%v", err)
	}
	for l, want := range map[string]string{
		"[owner-repo-of-services, other]": "dictionary order",
		"[other, other]":                  "no duplicates",
		"[nobody, other]":                 `"nobody" is not in adopt_rules`,
		"[]":                              "needs decision",
	} {
		if _, _, err := loadAdoptPair(t, two(l)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", l, err, want)
		}
	}
}

func TestAdoptGenFileWithoutItsHandFile(t *testing.T) {
	g, _ := os.ReadFile("testdata/adopt/go-fake.adopt.gen.yaml")
	gen, err := Parse(strings.NewReader(string(g)), "builtin:go-fake.adopt.gen.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckAdoption(gen); err == nil || !strings.Contains(err.Error(), "is not loaded with it") {
		t.Fatalf("%v", err)
	}
}
