package taint

import (
	"testing"

	"github.com/yagihash/rivus/internal/testutil"
)

// A finding whose path goes through a write via a pointer that rivus approximates by aliasing (ptrAlias, Rust only
// today) carries the pointer_alias note; a finding in the same program whose path does not, carries none. The
// program is Go, with sg.Program.HasEnv turned on, so that the mechanism runs on it.
func TestPointerAliasNoteOnlyOnThePathThatUsedTheAlias(t *testing.T) {
	run := func(hasEnv bool) *Result {
		prog := loadIR(t, testutil.Testdata(), "./cases/ptralias/...")
		prog.sg.HasEnv = hasEnv
		rules := testutil.Rules(t)
		rules.Expand(prog.gossa, 0)
		return prog.engine(prog.sites(rules), rules, DefaultOptions()).Run()
	}
	noteOf := func(res *Result, fn string) (found, noted bool) {
		for _, f := range res.Findings {
			if f.Sink.Func == "example.com/cases/cases/ptralias."+fn || f.Source.Func == "example.com/cases/cases/ptralias."+fn {
				found = true
				for _, u := range f.Uncertainties {
					if u.Kind == KindPointerAlias {
						noted = true
						if u.AttributableTo != AttributableRivus {
							t.Errorf("attributable_to = %q", u.AttributableTo)
						}
					}
				}
			}
		}
		return
	}
	res := run(true)
	if found, noted := noteOf(res, "ThroughSlot"); !found || !noted {
		t.Errorf("ThroughSlot: found %v, pointer_alias %v: the flow is the aliasing's, and says so", found, noted)
	}
	if found, noted := noteOf(res, "Direct"); !found || noted {
		t.Errorf("Direct: found %v, pointer_alias %v: no aliasing in its path", found, noted)
	}
	// without the mechanism no finding has the note
	for _, f := range run(false).Findings {
		for _, u := range f.Uncertainties {
			if u.Kind == KindPointerAlias {
				t.Errorf("a finding has the note with no ptrAlias: %+v", f.Sink.Position)
			}
		}
	}
}
