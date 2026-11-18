package taint

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/testutil"
)

// A finding says whether its path can be reached from where the program starts.
func TestReachabilityFromEntryPoints(t *testing.T) {
	prog := loadIR(t, testutil.Testdata(), "./cases/reach")
	rules := testutil.Rules(t)
	st := prog.sites(rules)
	res := prog.engine(st, rules, DefaultOptions()).Run()
	want := map[string]string{
		"reach/main.go:12": ReachFromEntry,    // used, called by main
		"reach/main.go:18": ReachNotFromEntry, // orphan, called by nothing
		"reach/main.go:25": ReachFromEntry,    // thing.Run, through the interface called from init
	}
	got := map[string]string{}
	for _, f := range res.Findings {
		p := f.Sink.Position
		got[strings.TrimPrefix(p.File, "cases/")+":"+itoa(p.Line)] = f.Reachability
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: reachability %q, want %q (all: %v)", k, got[k], w, got)
		}
	}
}
