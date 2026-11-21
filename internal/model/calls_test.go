package model

import (
	"strings"
	"testing"
)

func parseCalls(t *testing.T, text string) (*RuleSet, error) {
	t.Helper()
	return Parse(strings.NewReader(text), "test.yaml")
}

// A row of calls is read, and a row that returns something has what stands for it when the function that is
// called cannot be told.
func TestCallsRowsAreReadAndDerive(t *testing.T) {
	rs, err := parseCalls(t, `
propagators:
  - symbol: a.Map
    calls:
      - {fn: arg0, with: [recv], ret: result0, throw: result1, writes: {0: arg1}}
  - symbol: a.Once
    calls: [{fn: arg0}]
  - symbol: a.Go
    pure: true
    calls: [{fn: arg0, mode: go}]
  - symbol: a.Err
    flows: [{from: recv, to: result0}]
    calls: [{fn: arg0, with: [recv], ret: result0}]
`)
	if err != nil {
		t.Fatal(err)
	}
	m := rs.Propagators[0]
	d := m.DerivedFlows(0)
	if len(d) != 1 || d[0].From != "recv" || d[0].To != "result0" || d[0].Assumed == "" {
		t.Errorf("derived flows of the row: %+v", d)
	}
	if got := rs.Propagators[1].DerivedFlows(0); len(got) != 0 {
		t.Errorf("a row without ret carries nothing: %+v", got)
	}
	if got := rs.Propagators[2].DerivedFlows(0); len(got) != 0 {
		t.Errorf("a callback that runs on its own has no result: %+v", got)
	}
	r := Merge(rs)
	if !r.Calls["a.Map"] || r.Calls["a.Nothing"] {
		t.Errorf("Calls index: %v", r.Calls)
	}
}

func TestCallsRowErrors(t *testing.T) {
	cases := map[string]string{
		"no fn":                "propagators:\n  - symbol: a.B\n    calls: [{with: [recv]}]\n",
		"fn is a result":       "propagators:\n  - symbol: a.B\n    calls: [{fn: result0}]\n",
		"fn is all args":       "propagators:\n  - symbol: a.B\n    calls: [{fn: args}]\n",
		"bad with":             "propagators:\n  - symbol: a.B\n    calls: [{fn: arg0, with: [nope]}]\n",
		"with is a result":     "propagators:\n  - symbol: a.B\n    calls: [{fn: arg0, with: [result0]}]\n",
		"bad ret":              "propagators:\n  - symbol: a.B\n    calls: [{fn: arg0, with: [recv], ret: nope}]\n",
		"bad throw":            "propagators:\n  - symbol: a.B\n    calls: [{fn: arg0, with: [recv], throw: nope}]\n",
		"bad mode":             "propagators:\n  - symbol: a.B\n    calls: [{fn: arg0, mode: later}]\n",
		"negative writes":      "propagators:\n  - symbol: a.B\n    calls: [{fn: arg0, writes: {-1: recv}}]\n",
		"ret without with":     "propagators:\n  - symbol: a.B\n    calls: [{fn: arg0, ret: result0}]\n",
		"ret of a background":  "propagators:\n  - symbol: a.B\n    calls: [{fn: arg0, with: [recv], ret: result0, mode: go}]\n",
		"unknown key in a row": "propagators:\n  - symbol: a.B\n    calls: [{fn: arg0, feed: []}]\n",
		"pure with flows":      "propagators:\n  - symbol: a.B\n    pure: true\n    flows: [{from: recv, to: result}]\n    calls: [{fn: arg0}]\n",
	}
	for name, text := range cases {
		if _, err := parseCalls(t, text); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// ret without with is fine when flows say where the result comes from
	if _, err := parseCalls(t, "propagators:\n  - symbol: a.B\n    flows: [{from: recv, to: result0}]\n    calls: [{fn: arg0, ret: result0}]\n"); err != nil {
		t.Errorf("ret with flows: %v", err)
	}
}
