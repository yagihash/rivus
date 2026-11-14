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
		"unknown field":      "ruleset: x\nbogus: 1\n",
		"source no labels":   "sources:\n  - symbol: a.B\n",
		"source no symbol":   "sources:\n  - labels: [x]\n",
		"sink no symbol":     "sinks:\n  - category: x\n",
		"sanitizer no drops": "sanitizers:\n  - symbol: a.B\n",
		"propagator no flow": "propagators:\n  - symbol: a.B\n",
		"bad selector":       "propagators:\n  - symbol: a.B\n    flows: [{from: nope, to: result}]\n",
		"implementations":    "sinks:\n  - symbol: a.B\n    match: implementations\n",
		"unknown match":      "sinks:\n  - symbol: a.B\n    match: fuzzy\n",
		"unknown kind":       "sources:\n  - kind: magic\n    labels: [x]\n",
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
