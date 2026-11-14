package taint

import (
	"os"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/callgraph"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
	"github.com/yagihash/rivus/internal/sites"
)

// TestRealCode runs the analysis on a real module to check that it neither
// crashes nor blows up. There is no ground truth for such code, so the test
// checks robustness, not detections. Set RIVUS_TEST_MODULE to a module
// directory to run it, for example the ghmint checkout.
func TestRealCode(t *testing.T) {
	dir := os.Getenv("RIVUS_TEST_MODULE")
	if dir == "" {
		t.Skip("RIVUS_TEST_MODULE is not set")
	}
	prog, err := gossa.Load(dir, []string{"./..."}, gossa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	rules := model.Merge(b...)
	st := sites.Enumerate(prog, rules)
	start := time.Now()
	res := New(prog, st, rules, callgraph.BuildGo(prog), DefaultOptions()).Run()
	elapsed := time.Since(start)
	t.Logf("%d sites, %d functions, %d findings, %d uncertainties, %d passes, %v",
		len(st.Sites), res.Stats.Functions, len(res.Findings), len(res.Uncertainties), res.Stats.Passes, elapsed)
	if !res.Stats.Converged {
		t.Errorf("did not converge: %+v", res.Stats)
	}
	if elapsed > 60*time.Second {
		t.Errorf("too slow: %v", elapsed)
	}
	for _, f := range res.Findings {
		if len(f.Paths) == 0 || len(f.Paths[0].Steps) == 0 {
			t.Errorf("finding without a path: %s", f.ID)
		}
		for _, s := range f.Paths[0].Steps {
			if s.Position.Line == 0 && len(s.Notes) == 0 {
				t.Errorf("step without position: %+v", s)
			}
		}
	}
}
