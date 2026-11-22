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
		"unknown field":                   "ruleset: x\nbogus: 1\n",
		"source no labels":                "sources:\n  - symbol: a.B\n",
		"source no symbol":                "sources:\n  - labels: [x]\n",
		"sink no symbol":                  "sinks:\n  - category: x\n",
		"sanitizer no drops":              "sanitizers:\n  - symbol: a.B\n",
		"propagator no flow":              "propagators:\n  - symbol: a.B\n",
		"bad selector":                    "propagators:\n  - symbol: a.B\n    flows: [{from: nope, to: result}]\n",
		"sanitizer expansion":             "sanitizers:\n  - symbol: (a.I).M\n    drops: [x]\n    match: implementations\n",
		"unknown match":                   "sinks:\n  - symbol: a.B\n    match: fuzzy\n",
		"unknown kind":                    "sources:\n  - kind: magic\n    labels: [x]\n",
		"sink negative arg":               "sinks:\n  - symbol: a.B\n    args: [-1, 7]\n",
		"sink when_arg_type no types":     "sinks:\n  - symbol: a.B\n    when_arg_type: {arg: 0}\n",
		"sink when_arg_type negative":     "sinks:\n  - symbol: a.B\n    when_arg_type: {arg: -1, types: [x]}\n",
		"sink when_arg_type recv and arg": "sinks:\n  - symbol: a.B\n    when_arg_type: {recv: true, arg: 1, types: [x]}\n",
		"param negative":                  "sources:\n  - kind: param\n    symbol: a.B\n    params: [-1]\n    labels: [x]\n",
		"field with symbol":               "sources:\n  - kind: field\n    field: a.T.f\n    symbol: a.B\n    labels: [x]\n",
		"field with params":               "sources:\n  - kind: field\n    field: a.T.f\n    params: [0]\n    labels: [x]\n",
		"field with out":                  "sources:\n  - kind: field\n    field: a.T.f\n    out: [0]\n    labels: [x]\n",
		"field implementations":           "sources:\n  - kind: field\n    field: a.T.f\n    match: implementations\n    labels: [x]\n",
		"param_type with symbol":          "sources:\n  - kind: param_type\n    type: a.T\n    symbol: a.B\n    labels: [x]\n",
		"param_type implementations":      "sources:\n  - kind: param_type\n    type: a.T\n    match: implementations\n    labels: [x]\n",
		"call_result with params":         "sources:\n  - symbol: a.B\n    params: [0]\n    labels: [x]\n",
		"callback_param no operand":       "sources:\n  - kind: callback_param\n    symbol: a.B\n    labels: [x]\n",
		"callback_param negative operand": "sources:\n  - kind: callback_param\n    symbol: a.B\n    operand: -1\n    labels: [x]\n",
		"callback_param negative from":    "sources:\n  - kind: callback_param\n    symbol: a.B\n    operand: 0\n    from: -1\n    labels: [x]\n",
		"callback_param to before from":   "sources:\n  - kind: callback_param\n    symbol: a.B\n    operand: 0\n    from: 2\n    to: 1\n    labels: [x]\n",
		"callback_param no symbol":        "sources:\n  - kind: callback_param\n    operand: 0\n    labels: [x]\n",
		"callback_param with params":      "sources:\n  - kind: callback_param\n    symbol: a.B\n    operand: 0\n    params: [0]\n    labels: [x]\n",
		"callback_param implementations":  "sources:\n  - kind: callback_param\n    symbol: a.B\n    operand: 0\n    match: implementations\n    labels: [x]\n",
		"operand on param":                "sources:\n  - kind: param\n    symbol: a.B\n    params: [0]\n    operand: 0\n    labels: [x]\n",
		"from on call_result":             "sources:\n  - symbol: a.B\n    from: 1\n    labels: [x]\n",
		"second document":                 "ruleset: t\nsources:\n  - symbol: a.B\n    labels: [x]\n---\nruleset: t2\nsinks:\n  - symbol: a.C\n    category: x\n  - this_is: garbage\n",
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

func TestSinkParts(t *testing.T) {
	bad := map[string]string{
		"field without a position":    "sinks:\n  - symbol: a.B\n    field: X\n",
		"field with two positions":    "sinks:\n  - symbol: a.B\n    args: [0, 1]\n    field: X\n",
		"field and parts":             "sinks:\n  - symbol: a.B\n    args: [0]\n    field: X\n    parts: [{arg: 0, field: Y}]\n",
		"part without a position":     "sinks:\n  - symbol: a.B\n    parts: [{field: X}]\n",
		"part arg and recv":           "sinks:\n  - symbol: a.B\n    parts: [{arg: 0, recv: true, field: X}]\n",
		"part negative arg":           "sinks:\n  - symbol: a.B\n    parts: [{arg: -1, field: X}]\n",
		"part without a field":        "sinks:\n  - symbol: a.B\n    parts: [{arg: 0}]\n",
		"part with an empty name":     "sinks:\n  - symbol: a.B\n    parts: [{arg: 0, field: \"A..B\"}]\n",
		"part with a pointer step":    "sinks:\n  - symbol: a.B\n    parts: [{arg: 0, field: \"A.*.B\"}]\n",
		"part with an unknown member": "sinks:\n  - symbol: a.B\n    parts: [{arg: 0, field: A, path: B}]\n",
	}
	for name, src := range bad {
		if _, err := Parse(strings.NewReader(src), name); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	rs, err := Parse(strings.NewReader(`sinks:
  - symbol: a.Short
    args: [1]
    field: A.B
  - symbol: a.Recv
    recv: true
    field: Path
  - symbol: a.Parts
    args: [0]
    parts:
      - {arg: 1, field: "[].SQL"}
      - {recv: true, field: X}
  - symbol: a.Plain
    args: [2]
`), "ok")
	if err != nil {
		t.Fatal(err)
	}
	short, recv, parts, plain := rs.Sinks[0], rs.Sinks[1], rs.Sinks[2], rs.Sinks[3]
	if p := short.PartList(); len(p) != 1 || p[0].Arg == nil || *p[0].Arg != 1 || p[0].Field != "A.B" {
		t.Errorf("short form: %+v", p)
	}
	if got := short.WholeArgs(3); len(got) != 0 || short.WholeRecv() {
		t.Errorf("the short form reads its position by the part only: %v %v", got, short.WholeRecv())
	}
	if p := recv.PartList(); len(p) != 1 || !p[0].Recv || p[0].Field != "Path" || recv.WholeRecv() {
		t.Errorf("recv short form: %+v", p)
	}
	if got := parts.WholeArgs(3); len(got) != 1 || got[0] != 0 {
		t.Errorf("args beside parts are whole, and no other argument is: %v", got)
	}
	if got := plain.WholeArgs(3); len(got) != 1 || got[0] != 2 {
		t.Errorf("a sink without parts is as it was: %v", got)
	}
	if (&Sink{}).HasParts() {
		t.Error("no parts")
	}
	if got := (&Sink{}).WholeArgs(2); len(got) != 2 {
		t.Errorf("no args and no parts is every argument: %v", got)
	}
	if got := (&Sink{Parts: []SinkPart{{Recv: true, Field: "X"}}}).WholeArgs(2); len(got) != 0 {
		t.Errorf("parts and no args is not every argument: %v", got)
	}
}

func TestSinkNamed(t *testing.T) {
	bad := map[string]string{
		"empty name":             "sinks:\n  - symbol: a.B\n    named: [\"\"]\n",
		"name with a dot":        "sinks:\n  - symbol: a.B\n    named: [\"x.y\"]\n",
		"short form, two names":  "sinks:\n  - symbol: a.B\n    named: [x, y]\n    field: X\n",
		"short form, arg+named":  "sinks:\n  - symbol: a.B\n    args: [0]\n    named: [x]\n    field: X\n",
		"part arg and named":     "sinks:\n  - symbol: a.B\n    parts: [{arg: 0, named: x, field: X}]\n",
		"part recv and named":    "sinks:\n  - symbol: a.B\n    parts: [{recv: true, named: x, field: X}]\n",
		"part with a bad name":   "sinks:\n  - symbol: a.B\n    parts: [{named: \"1x\", field: X}]\n",
		"part named w/o a field": "sinks:\n  - symbol: a.B\n    parts: [{named: x}]\n",
	}
	for name, src := range bad {
		if _, err := Parse(strings.NewReader(src), name); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	rs, err := Parse(strings.NewReader("sinks:\n  - symbol: a.B\n    named: [kind]\n  - symbol: a.C\n    named: [r]\n    field: url\n  - symbol: a.D\n    parts: [{named: r, field: url}]\n"), "ok")
	if err != nil {
		t.Fatal(err)
	}
	if got := rs.Sinks[0].WholeArgs(3); len(got) != 0 {
		t.Errorf("named and no args is not every argument: %v", got)
	}
	if p := rs.Sinks[1].PartList(); len(p) != 1 || p[0].Named != "r" || p[0].Field != "url" {
		t.Errorf("short form by name: %+v", p)
	}
}

func TestCallbackParamIsIndexedBySymbolWithItsSetsCounting(t *testing.T) {
	a, err := Parse(strings.NewReader("sources:\n  - kind: callback_param\n    symbol: a.Route\n    operand: 1\n    from: 1\n    to: 2\n    labels: [user_input]\n"), "a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse(strings.NewReader("self_first: true\nsources:\n  - kind: callback_param\n    symbol: a.Rs\n    operand: 1\n    labels: [user_input]\n"), "b")
	if err != nil {
		t.Fatal(err)
	}
	r := Merge(a, b)
	g := r.CallbackParams["a.Route"]
	if len(g) != 1 || *g[0].Operand != 1 || *g[0].From != 1 || *g[0].To != 2 || g[0].SelfFirst {
		t.Errorf("a.Route: %+v", g)
	}
	if g := r.CallbackParams["a.Rs"]; len(g) != 1 || !g[0].SelfFirst || g[0].From != nil {
		t.Errorf("a.Rs: %+v", g)
	}
	if len(r.ParamFuncs)+len(r.ParamTypes)+len(r.CallSources) != 0 {
		t.Errorf("a callback_param is no other kind of source")
	}
}
