package sites

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
	"github.com/yagihash/rivus/internal/testutil"
)

// The IR path must give the sites that the go/ssa path gives: the same IDs,
// fingerprints, positions, snippets and marks (ADR-028 makes IDs something an
// agent keeps, so they cannot depend on the way the code was read).

func rulesWith(t *testing.T, files ...string) *model.Rules {
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

func describe(s *Site) string {
	lit := "-"
	if s.LiteralOnly != nil {
		lit = fmt.Sprint(*s.LiteralOnly)
	}
	return fmt.Sprintf("%s %s %s %s %s/%s %v %+v lit=%s snippet=%q fp=%v", s.ID, s.Role, s.Kind, s.Func, s.Symbol, s.Category, s.Labels, s.Position, lit, s.Snippet, s.Fingerprint)
}

type irStats struct{ sites, sinks, literal, fields, params int }

func compareSites(t *testing.T, dir string, patterns []string, rules *model.Rules) irStats {
	t.Helper()
	lifted, err := gossa.Load(dir, patterns, gossa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	naive, err := gossa.Load(dir, patterns, gossa.Options{NaiveForm: true})
	if err != nil {
		t.Fatal(err)
	}
	prog, err := goir.Extract(naive, goir.Options{All: true})
	if err != nil {
		t.Fatal(err)
	}
	old := Enumerate(lifted, rules)
	got := EnumerateIR(prog, goir.NewTexts(naive), rules)

	var st irStats
	want := map[string]string{}
	for _, s := range old.Sites {
		want[s.ID] = describe(s)
		st.sites++
		if s.Role == RoleSink {
			st.sinks++
			if s.LiteralOnly != nil && *s.LiteralOnly {
				st.literal++
			}
		}
		switch s.Kind {
		case model.KindField:
			st.fields++
		case model.KindParam, model.KindParamType:
			st.params++
		}
	}
	have := map[string]string{}
	for _, s := range got.Sites {
		have[s.ID] = describe(s)
	}
	var diffs []string
	for id, w := range want {
		if h, ok := have[id]; !ok {
			diffs = append(diffs, "missing in the IR path: "+w)
		} else if h != w {
			diffs = append(diffs, "differs:\n    go/ssa: "+w+"\n    IR:     "+h)
		}
	}
	for id, h := range have {
		if _, ok := want[id]; !ok {
			diffs = append(diffs, "only in the IR path: "+h)
		}
	}
	sort.Strings(diffs)
	if len(diffs) > 0 {
		if len(diffs) > 10 {
			diffs = append(diffs[:10], fmt.Sprintf("... and %d more", len(diffs)-10))
		}
		t.Errorf("%d sites on the go/ssa path, %d on the IR path:\n  %s", len(want), len(have), strings.Join(diffs, "\n  "))
	}
	// The order of the list is part of the answer, too.
	var a, b []string
	for _, s := range old.Sites {
		a = append(a, s.ID)
	}
	for _, s := range got.Sites {
		b = append(b, s.ID)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("the sites come in another order")
	}
	return st
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func TestIRPathGivesTheSameSitesOnTheTestCases(t *testing.T) {
	st := compareSites(t, testutil.Testdata(), []string{"./..."}, testutil.Rules(t))
	t.Logf("%+v", st)
	if st.sites < 30 || st.literal < 1 || st.params < 1 {
		t.Errorf("too little was compared: %+v", st)
	}
}

func TestIRPathGivesTheSameSitesOnRivusItself(t *testing.T) {
	rules := rulesWith(t, filepath.Join(repoRoot(), "dogfood", "rules.yaml"))
	st := compareSites(t, repoRoot(), []string{"./..."}, rules)
	t.Logf("%+v", st)
	if st.sites < 10 {
		t.Errorf("too little was compared: %+v", st)
	}
}

func TestIRPathGivesTheSameSitesOnAModule(t *testing.T) {
	dir := os.Getenv("RIVUS_TEST_MODULE")
	if dir == "" {
		t.Skip("RIVUS_TEST_MODULE is not set")
	}
	st := compareSites(t, dir, []string{"./..."}, rulesWith(t))
	t.Logf("%+v", st)
}
