package model

import (
	"strings"
	"testing"
)

func TestParseSelector(t *testing.T) {
	good := map[string]Selector{
		"recv":    {Kind: SelRecv},
		"arg0":    {Kind: SelArg},
		"arg12":   {Kind: SelArg, Index: 12},
		"args":    {Kind: SelArg, All: true},
		"result":  {Kind: SelResult},
		"result1": {Kind: SelResult, Index: 1},
		"results": {Kind: SelResult, All: true},
	}
	for in, want := range good {
		got, err := ParseSelector(in)
		if err != nil || got != want {
			t.Errorf("ParseSelector(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "arg", "arg-1", "argx", "res", "receiver"} {
		if _, err := ParseSelector(in); err == nil {
			t.Errorf("ParseSelector(%q) succeeded", in)
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"unknown field":               "ruleset: x\nbogus: 1\n",
		"source no labels":            "sources:\n  - symbol: a.B\n",
		"source no symbol":            "sources:\n  - labels: [x]\n",
		"sink no symbol":              "sinks:\n  - category: x\n",
		"sanitizer no drops":          "sanitizers:\n  - symbol: a.B\n",
		"propagator no flow":          "propagators:\n  - symbol: a.B\n",
		"bad selector":                "propagators:\n  - symbol: a.B\n    flows: [{from: nope, to: result}]\n",
		"sanitizer expansion":         "sanitizers:\n  - symbol: (a.I).M\n    drops: [x]\n    match: implementations\n",
		"unknown match":               "sinks:\n  - symbol: a.B\n    match: fuzzy\n",
		"unknown kind":                "sources:\n  - kind: magic\n    labels: [x]\n",
		"sink negative arg":           "sinks:\n  - symbol: a.B\n    args: [-1, 7]\n",
		"sink when_arg_type no types": "sinks:\n  - symbol: a.B\n    when_arg_type: {arg: 0}\n",
		"sink when_arg_type negative": "sinks:\n  - symbol: a.B\n    when_arg_type: {arg: -1, types: [x]}\n",
		"sink when_arg_type recv and arg": "sinks:\n  - symbol: a.B\n    when_arg_type: {recv: true, arg: 1, types: [x]}\n",
		"param negative":              "sources:\n  - kind: param\n    symbol: a.B\n    params: [-1]\n    labels: [x]\n",
		"field with symbol":           "sources:\n  - kind: field\n    field: a.T.f\n    symbol: a.B\n    labels: [x]\n",
		"field with params":           "sources:\n  - kind: field\n    field: a.T.f\n    params: [0]\n    labels: [x]\n",
		"field with out":              "sources:\n  - kind: field\n    field: a.T.f\n    out: [0]\n    labels: [x]\n",
		"field implementations":       "sources:\n  - kind: field\n    field: a.T.f\n    match: implementations\n    labels: [x]\n",
		"param_type with symbol":      "sources:\n  - kind: param_type\n    type: a.T\n    symbol: a.B\n    labels: [x]\n",
		"param_type implementations":  "sources:\n  - kind: param_type\n    type: a.T\n    match: implementations\n    labels: [x]\n",
		"call_result with params":     "sources:\n  - symbol: a.B\n    params: [0]\n    labels: [x]\n",
		"second document":             "ruleset: t\nsources:\n  - symbol: a.B\n    labels: [x]\n---\nruleset: t2\nsinks:\n  - symbol: a.C\n    category: x\n  - this_is: garbage\n",
	}
	for name, src := range cases {
		if _, err := Parse(strings.NewReader(src), name); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestMerge(t *testing.T) {
	a, err := Parse(strings.NewReader(`
sources:
  - symbol: a.Src
    labels: [user_input]
sinks:
  - symbol: a.Sink
    args: [0]
`), "a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse(strings.NewReader(`
sinks:
  - symbol: a.Sink
    accepts: [user_input]
sanitizers:
  - symbol: a.San
    drops: ["*"]
`), "b")
	if err != nil {
		t.Fatal(err)
	}
	r := Merge(a, b)
	if len(r.CallSources["a.Src"]) != 1 || r.CallSources["a.Src"][0].Kind != KindCallResult {
		t.Errorf("call source not indexed: %+v", r.CallSources)
	}
	if len(r.Sinks["a.Sink"]) != 2 {
		t.Errorf("sink entries should accumulate: %+v", r.Sinks)
	}
	if !r.HasModel("a.San") || r.HasModel("a.Sink") {
		t.Errorf("HasModel is wrong")
	}
}

func TestPureModel(t *testing.T) {
	rs, err := Parse(strings.NewReader("propagators:\n  - symbol: a.Lock\n    pure: true\n"), "t")
	if err != nil {
		t.Fatal(err)
	}
	r := Merge(rs)
	if !r.HasModel("a.Lock") {
		t.Error("a pure function is modelled")
	}
	for name, src := range map[string]string{
		"neither":     "propagators:\n  - symbol: a.F\n",
		"contradicts": "propagators:\n  - symbol: a.F\n    pure: true\n    flows: [{from: arg0, to: result}]\n",
	} {
		if _, err := Parse(strings.NewReader(src), name); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestOrigins(t *testing.T) {
	rs, err := Parse(strings.NewReader("sinks:\n  - symbol: a.S\n"), "rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if rs.Sinks[0].Origin != OriginHuman {
		t.Errorf("a user rule defaults to human, got %q", rs.Sinks[0].Origin)
	}
	b, err := Parse(strings.NewReader("sinks:\n  - symbol: a.S\n"), "builtin:x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if b.Sinks[0].Origin != OriginBuiltin {
		t.Errorf("a built-in rule defaults to builtin, got %q", b.Sinks[0].Origin)
	}
	d, err := Parse(strings.NewReader("origin: llm\nsinks:\n  - symbol: a.S\n    decided_at: \"2026-11-20\"\n    rationale: r\n    model_version: m\n  - symbol: a.T\n    origin: human\n"), "x")
	if err != nil {
		t.Fatal(err)
	}
	if d.Sinks[0].Origin != OriginLLM || d.Sinks[1].Origin != OriginHuman {
		t.Errorf("origins = %q, %q", d.Sinks[0].Origin, d.Sinks[1].Origin)
	}
}

func TestLLMRulesNeedTheirWhy(t *testing.T) {
	for name, src := range map[string]string{
		"nothing":      "sinks:\n  - symbol: a.S\n    origin: llm\n",
		"no rationale": "sinks:\n  - symbol: a.S\n    origin: llm\n    decided_at: d\n    model_version: m\n",
		"no version":   "sinks:\n  - symbol: a.S\n    origin: llm\n    decided_at: d\n    rationale: r\n",
		"unknown":      "sinks:\n  - symbol: a.S\n    origin: oracle\n",
	} {
		if _, err := Parse(strings.NewReader(src), name); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestWithoutOriginLeavesTheLLMRulesOut(t *testing.T) {
	rs, err := Parse(strings.NewReader(`
sanitizers:
  - symbol: a.Human
    drops: [x]
  - symbol: a.Machine
    drops: [x]
    origin: llm
    decided_at: d
    rationale: r
    model_version: m
propagators:
  - symbol: a.P
    pure: true
    origin: llm
    decided_at: d
    rationale: r
    model_version: m
`), "x")
	if err != nil {
		t.Fatal(err)
	}
	r := WithoutOrigin(OriginLLM, rs)
	if !r.DeterministicOnly || r.IgnoredModels != 2 {
		t.Errorf("deterministic = %v, ignored = %d", r.DeterministicOnly, r.IgnoredModels)
	}
	if !r.HasModel("a.Human") || r.HasModel("a.Machine") || r.HasModel("a.P") {
		t.Error("only the rules decided by an LLM must be gone")
	}
}

// A trailing `---`, or one before the rules, is not a second document of rules.
func TestParseEmptyExtraDocuments(t *testing.T) {
	for name, src := range map[string]string{
		"trailing":     "ruleset: t\nsources:\n  - symbol: a.B\n    labels: [x]\n---\n",
		"with comment": "ruleset: t\nsources:\n  - symbol: a.B\n    labels: [x]\n---\n# nothing\n",
	} {
		if _, err := Parse(strings.NewReader(src), name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
