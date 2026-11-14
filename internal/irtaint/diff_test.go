package irtaint_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/callgraph"
	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/irtaint"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
	"github.com/yagihash/rivus/internal/testutil"
)

// The IR path must give the result that the go/ssa path gives (ADR-039): the
// same findings with the same paths, the same uncertainties, the same
// suppressions. The comparison is on the JSON that a user gets.

type outcome struct {
	Findings      []json.RawMessage `json:"findings"`
	Uncertainties []json.RawMessage `json:"uncertainties"`
	Suppressions  []json.RawMessage `json:"suppressions"`
	Stats         struct {
		Converged bool `json:"converged"`
	} `json:"stats"`
}

func marshal(t *testing.T, v any) outcome {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var o outcome
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatal(err)
	}
	return o
}

func rulesFrom(t *testing.T, files ...string) *model.Rules {
	t.Helper()
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		rs, err := model.Parse(fh, f)
		fh.Close()
		if err != nil {
			t.Fatal(err)
		}
		sets = append(sets, rs)
	}
	return model.Merge(sets...)
}

func runOld(t *testing.T, dir string, patterns []string, rules *model.Rules) outcome {
	t.Helper()
	p, err := gossa.Load(dir, patterns, gossa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	st := sites.Enumerate(p, rules)
	res := taint.New(p, st, rules, callgraph.BuildGo(p), taint.DefaultOptions()).Run()
	return marshal(t, res)
}

func runNew(t *testing.T, dir string, patterns []string, rules *model.Rules) outcome {
	t.Helper()
	p, err := gossa.Load(dir, patterns, gossa.Options{NaiveForm: true, WithLifted: true})
	if err != nil {
		t.Fatal(err)
	}
	prog, err := goir.Extract(p, goir.Options{All: true})
	if err != nil {
		t.Fatal(err)
	}
	texts := goir.NewTexts(p)
	g, err := sg.Build(prog)
	if err != nil {
		t.Fatal(err)
	}
	st := sites.EnumerateIR(prog, texts, rules)
	e := irtaint.New(g, texts, st, rules, irtaint.DefaultOptions())
	e.SetLoadErrors(p.LoadErrors)
	return marshal(t, e.Run())
}

func keyed(t *testing.T, list []json.RawMessage, key func(m map[string]any) string) map[string]string {
	out := map[string]string{}
	for _, raw := range list {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		k := key(m)
		for {
			if _, dup := out[k]; !dup {
				break
			}
			k += "'"
		}
		out[k] = string(raw)
	}
	return out
}

func compareList(t *testing.T, what string, a, b []json.RawMessage, key func(m map[string]any) string) {
	t.Helper()
	ka, kb := keyed(t, a, key), keyed(t, b, key)
	var diffs []string
	for k, v := range ka {
		w, ok := kb[k]
		switch {
		case !ok:
			diffs = append(diffs, "only in the go/ssa result: "+k)
		case v != w:
			diffs = append(diffs, fmt.Sprintf("differs: %s\n      go/ssa: %s\n      IR:     %s", k, v, w))
		}
	}
	for k := range kb {
		if _, ok := ka[k]; !ok {
			diffs = append(diffs, "only in the IR result: "+k)
		}
	}
	sort.Strings(diffs)
	if len(diffs) > 0 {
		if len(diffs) > 6 {
			diffs = append(diffs[:6], fmt.Sprintf("... and %d more", len(diffs)-6))
		}
		t.Errorf("%s: %d on the go/ssa path, %d on the IR path\n  %s", what, len(a), len(b), strings.Join(diffs, "\n  "))
	}
}

func compare(t *testing.T, dir string, patterns []string, rules *model.Rules) (int, int) {
	t.Helper()
	old := runOld(t, dir, patterns, rules)
	nw := runNew(t, dir, patterns, rules)
	str := func(m map[string]any, keys ...string) string {
		var parts []string
		for _, k := range keys {
			if v, ok := m[k]; ok {
				parts = append(parts, fmt.Sprint(v))
			}
		}
		return strings.Join(parts, "|")
	}
	compareList(t, "findings", old.Findings, nw.Findings, func(m map[string]any) string { return str(m, "id") })
	compareList(t, "uncertainties", old.Uncertainties, nw.Uncertainties, func(m map[string]any) string {
		pos, _ := m["position"].(map[string]any)
		return str(m, "kind", "symbol", "func") + "|" + str(pos, "file", "offset")
	})
	compareList(t, "suppressions", old.Suppressions, nw.Suppressions, func(m map[string]any) string {
		pos, _ := m["position"].(map[string]any)
		return str(m, "func") + "|" + str(pos, "file", "offset")
	})
	if old.Stats.Converged != nw.Stats.Converged {
		t.Errorf("converged: %v on the go/ssa path, %v on the IR path", old.Stats.Converged, nw.Stats.Converged)
	}
	return len(old.Findings), len(old.Uncertainties)
}

func TestIRPathOnTheTestCases(t *testing.T) {
	rules := testutil.Rules(t)
	cases, err := filepath.Glob(filepath.Join(testutil.Testdata(), "cases", "*"))
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, c := range cases {
		name := filepath.Base(c)
		t.Run(name, func(t *testing.T) {
			f, u := compare(t, testutil.Testdata(), []string{"./cases/" + name}, rules)
			total += f
			t.Logf("%d findings, %d uncertainties", f, u)
		})
	}
	if total < 20 {
		t.Errorf("only %d findings were compared", total)
	}
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func TestIRPathOnRivusItself(t *testing.T) {
	rules := rulesFrom(t, filepath.Join(repoRoot(), "dogfood", "rules.yaml"))
	f, u := compare(t, repoRoot(), []string{"./..."}, rules)
	t.Logf("%d findings, %d uncertainties", f, u)
}

func TestIRPathOnAModule(t *testing.T) {
	dir := os.Getenv("RIVUS_TEST_MODULE")
	if dir == "" {
		t.Skip("RIVUS_TEST_MODULE is not set")
	}
	f, u := compare(t, dir, []string{"./..."}, rulesFrom(t))
	t.Logf("%d findings, %d uncertainties", f, u)
}
