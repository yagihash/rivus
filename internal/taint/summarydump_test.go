package taint

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/testutil"
)

// The dump of the summaries (bench/compare.sh --graph-check) has one line for each function, in the order of the IDs,
// with the number of elements and steps, and the same program gives the same bytes in two runs.
func TestSummaryDumpIsOrderedAndStable(t *testing.T) {
	rules := testutil.Rules(t)
	dump := func() (string, *Engine) {
		l := loadIR(t, testutil.Testdata(), "./cases/inter/...")
		e := l.engine(l.sites(rules), rules, DefaultOptions())
		e.Run()
		var b bytes.Buffer
		if err := e.WriteSummaryDump(&b); err != nil {
			t.Fatal(err)
		}
		return b.String(), e
	}
	a, e := dump()
	b, _ := dump()
	if a != b {
		t.Fatalf("the same program gives two dumps")
	}
	lines := strings.Split(strings.TrimSpace(a), "\n")
	if len(lines) == 0 || len(lines) > len(e.summaries) {
		t.Fatalf("%d lines for %d summaries", len(lines), len(e.summaries))
	}
	for i, l := range lines {
		if f := strings.Split(l, "\t"); len(f) != 4 || !strings.HasPrefix(f[1], "elems=") || !strings.HasPrefix(f[2], "steps=") || len(f[3]) != 64 {
			t.Errorf("a line of the wrong form: %q", l)
		}
		if i > 0 && lines[i-1] >= l {
			t.Errorf("not in the order of the IDs: %q before %q", lines[i-1], l)
		}
	}
	// the counts of the lines are the ones of the stats
	elems, steps := 0, 0
	for _, sum := range e.summaries {
		_, n, s := summaryText(sum)
		elems, steps = elems+n, steps+s
	}
	if st := e.Run().Stats; st.SummaryElems != elems || st.SummarySteps != steps {
		t.Errorf("the dump counts %d elements and %d steps, the stats %d and %d", elems, steps, st.SummaryElems, st.SummarySteps)
	}
}
