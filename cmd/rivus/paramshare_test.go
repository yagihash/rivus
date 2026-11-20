package main

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// TestParamSourceSharedObject records what a `kind: param` source does when
// several functions get the same object (testdata/go/cases/paramshare). A param
// source is a statement about the value the function received: the taint stays
// in the function and is not written back to the caller (ADR-048), so a second
// function that gets the same object does not read it. Before ADR-048 the
// parameter tainted the object and the taint was written back (OutParam /
// OutGlobal), and the finding "runB -> sink in runA" (CORE-A) was false.
//
// A row with wrong set says what is still wrong; when it is fixed the row
// fails, and the row (and the wrong note) is to be corrected.
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
		// source (offSourcePath, BENCH-2): a ceiling, today's value.
		off int
	}
	rows := []row{
		// runB is the only source and does not reach the sink; runA reaches it but is
		// not a source: no finding (ADR-048; CORE-A before)
		{pkg: "ptr", want: map[string]bool{}},
		{pkg: "local", want: map[string]bool{}},
		{pkg: "global", want: map[string]bool{}},
		// 2 real flows. Before ADR-048 the path of runB had no step in runB (it went
		// through runA, CORE-B); now each source stays in its own function.
		{pkg: "repro", want: map[string]bool{"runA": true, "runB": true}},
		// the shapes where it did not happen
		{pkg: "distinct", want: map[string]bool{"runA": true, "runB": true}},
		{pkg: "samelocal", want: map[string]bool{"runA": true, "runB": true}},
		{pkg: "str", want: map[string]bool{"runA": true, "runB": true}},
		// param_type: HandleC does not reach the sink; the finding of HandleA is
		// right and HandleC is no longer in other_sources (CORE-A before: a wrong
		// trace that made a reader believe another entry point also reaches it)
		{pkg: "ptype", want: map[string]bool{"HandleA": true}},
		// the router and the handlers: handlerB is a source and only holds the
		// request, handlerA reaches the sink; no finding for handlerB
		{pkg: "router", want: map[string]bool{"handlerA": true}},
		// a write the function makes itself is a flow, to a global (also through a
		// helper, and a channel) and to the state behind a parameter: 1 finding each
		// (CORE-B: the path of a flow through a global has no step in the function that
		// wrote it, today as before ADR-048)
		{pkg: "gwrite", want: map[string]bool{"handler": false}, off: 1},
		{pkg: "gwritehelper", want: map[string]bool{"handler": false}, off: 1},
		{pkg: "gqueue", want: map[string]bool{"handler": false}, off: 1},
		{pkg: "ctxstate", want: map[string]bool{"handler": true}},
		// a source function that only takes the value in: x_direct (the flow from the
		// parameter itself to a sink in the same function is kept)
		{pkg: "direct", want: map[string]bool{"runB": true}},
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

// TestParamSourceNotWrittenBackIsLost: what ADR-048 gives up. runB is a source
// that only holds the request; the caller then uses the same request in a sink.
// KNOWN MISS (silent): the finding "the request runB received reaches
// os.Create in Main" is not made, because a param source is a statement about
// the value the function received and is not written back to the caller.
func TestParamSourceNotWrittenBackIsLost(t *testing.T) {
	var rep report.Report
	if err := json.Unmarshal(rivus(t, "analyze", "-rules", "rules.yaml", "./cases/paramshare/lost"), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("the known miss is found now (%d findings): update ADR-048 and make this test fail on a miss", len(rep.Findings))
	}
	t.Skip("known miss (ADR-048): the taint of a param source is not written back to the caller")
}
