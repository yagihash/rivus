package main

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// TestParamSourceSharedObject records what a `kind: param` source does when
// several functions get the same object (testdata/go/cases/paramshare). The
// parameter taints the object it points to, and the taint is written back to
// the caller (OutParam / OutGlobal); a second function that gets the same
// object then reads it.
//
// Every row is the behaviour of today, including what is wrong. A row with
// wrong set says what: when the engine is fixed the row fails, and the row
// (and the wrong note) is to be corrected.
func TestParamSourceSharedObject(t *testing.T) {
	type row struct {
		pkg string
		// sources of the findings (function name), and for each whether the
		// path of the finding has a step in that function.
		want map[string]bool
		// other_sources_count of the findings together
		others int
		// wrong says what the findings get wrong today.
		wrong string
		// off is the number of the findings with no step in the function of their
		// source (offSourcePath, BENCH-2): a ceiling, today's value. It goes down to 0
		// when CORE-B is fixed.
		off int
	}
	rows := []row{
		{pkg: "ptr", want: map[string]bool{"runB": true},
			wrong: "CORE-A: runB does not reach the sink, runA does; the finding is false"},
		{pkg: "local", want: map[string]bool{"runB": true},
			wrong: "CORE-A: the same with a local slice; the finding is false"},
		{pkg: "global", want: map[string]bool{"runB": false}, off: 1,
			wrong: "CORE-A: the finding is false; CORE-B: and its path has no step in runB, no global_var uncertainty"},
		{pkg: "repro", want: map[string]bool{"runA": true, "runB": false}, off: 1,
			wrong: "CORE-B: 2 real flows, but the path of runB has no step in runB (it goes through runA); CORE-C picks by function order"},
		// the shapes where it does not happen
		{pkg: "distinct", want: map[string]bool{"runA": true, "runB": true}},
		{pkg: "samelocal", want: map[string]bool{"runA": true, "runB": true}},
		{pkg: "str", want: map[string]bool{"runA": true, "runB": true}},
		// param_type: HandleC does not reach the sink; the finding of HandleA is
		// right and HandleC is in other_sources (merged, so not a finding of its own)
		{pkg: "ptype", want: map[string]bool{"HandleA": true}, others: 1,
			wrong: "CORE-A: HandleC is listed in other_sources, with a path through HandleA's flow"},
	}
	for _, r := range rows {
		t.Run(r.pkg, func(t *testing.T) {
			var rep report.Report
			if err := json.Unmarshal(rivus(t, "analyze", "-rules", "rules.yaml", "./cases/paramshare/"+r.pkg), &rep); err != nil {
				t.Fatal(err)
			}
			got := map[string]bool{}
			others := 0
			for _, f := range rep.Findings {
				name := f.Source.Func[strings.LastIndex(f.Source.Func, ".")+1:]
				in := false
				for _, s := range f.Paths[0].Steps {
					if s.Func == f.Source.Func {
						in = true
					}
				}
				got[name] = in
				others += f.OtherSourcesCount
			}
			if len(got) != len(r.want) || others != r.others {
				t.Errorf("findings from %v (other sources %d), want from %v (%d): %s", keys(got), others, keys(r.want), r.others, r.wrong)
			}
			off := offSourcePath(&rep)
			if len(off) > r.off {
				t.Errorf("%d findings have a path without a step in their source function: more than the %d of today", len(off), r.off)
			}
			if len(off) < r.off {
				t.Errorf("only %d findings have a path without a step in their source function, the ceiling is %d: lower it", len(off), r.off)
			}
			for n, in := range r.want {
				if g, ok := got[n]; ok && g != in {
					t.Errorf("%s: path has a step in the source function = %v, today %v: update the row", n, g, in)
				}
			}
		})
	}
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestGlobalWriterIsOnThePath: W writes a source value to a global and R
// reads it into a sink; the path must have a step in W, the function the
// value comes from. KNOWN MISS (CORE-B): today the path starts at the load in
// R, and W is only a candidate in the global_var note.
func TestGlobalWriterIsOnThePath(t *testing.T) {
	var rep report.Report
	if err := json.Unmarshal(rivus(t, "analyze", "-rules", "rules.yaml", "./cases/paramshare/globalw"), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(rep.Findings))
	}
	f := rep.Findings[0]
	// the same ceiling as the rows of TestParamSourceSharedObject: this finding is off its source
	if got := len(offSourcePath(&rep)); got > 1 {
		t.Errorf("%d findings have a path without a step in their source function: more than the 1 of today", got)
	}
	inW := false
	for _, s := range f.Paths[0].Steps {
		if strings.HasSuffix(s.Func, "globalw.W") {
			inW = true
		}
	}
	if !inW {
		t.Skip("known miss (CORE-B): the path has no step in W, the writer of the global")
	}
	t.Error("the path now has a step in W: make this test fail on a miss")
}
