package taint

import (
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
)

// The finding that stands for the sources of one flow is the best one to look
// at first, not the shortest (REVIEW-2026-11-19 CORE-3): the shortest path is
// often the one through a function value taken to reach every candidate.
func TestMergedSourcesShowTheCleanestPath(t *testing.T) {
	e := &Engine{sites: &sites.Result{ByID: map[string]*sites.Site{
		"a": {ID: "a", Kind: model.KindParam},
		"b": {ID: "b", Kind: model.KindParam},
		"c": {ID: "c", Kind: model.KindParam},
	}}}
	mk := func(src string, steps, uncertain, widened int, reach string) *Finding {
		f := &Finding{
			ID:              "f-" + src,
			Source:          SiteRef{ID: src, Symbol: "req", Func: "fn-" + src},
			Sink:            SiteRef{ID: "sink"},
			Category:        "x",
			Labels:          []string{"user_input"},
			Paths:           []Path{{Steps: make([]Step, steps)}},
			PathSteps:       steps,
			DispatchWidened: widened,
			Reachability:    reach,
		}
		for i := 0; i < uncertain; i++ {
			f.Uncertainties = append(f.Uncertainties, &Uncertainty{Kind: KindDynamicDispatch})
		}
		return f
	}
	// a: the shortest, through a widened call and not reachable; b: longer, clear and reachable
	got := e.mergeParamSources([]*Finding{
		mk("a", 3, 1, 1, ReachNotFromEntry),
		mk("b", 6, 0, 0, ReachFromEntry),
		mk("c", 4, 2, 1, ReachFromEntry),
	})
	if len(got) != 1 {
		t.Fatalf("%d findings", len(got))
	}
	if got[0].Source.ID != "b" {
		t.Errorf("the finding shown is %s, want b (the clear one)", got[0].Source.ID)
	}
	if got[0].OtherSourcesCount != 2 || len(got[0].OtherSources) != 2 {
		t.Fatalf("others: %+v", got[0].OtherSources)
	}
	for _, o := range got[0].OtherSources {
		if o.PathSteps == 0 {
			t.Errorf("%s: what is known of its path is gone: %+v", o.ID, o)
		}
	}
}
