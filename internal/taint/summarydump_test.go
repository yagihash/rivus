package taint

import (
	"bytes"
	"os"
	"path/filepath"
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

// The dump of each round (RIVUS_INTERNAL_SUMMARY_DUMP_EACH_ITER) writes one file for each round of a large component,
// the same bytes for any number of workers and in two runs, and the analysis gives the same summaries with it as
// without it.
func TestSummaryDumpEachIteration(t *testing.T) {
	rules := testutil.Rules(t)
	old := dumpMinSCC
	dumpMinSCC = parallelSCC // testdata/scc has one component of 14 functions
	defer func() { dumpMinSCC = old }()
	run := func(dir string, workers int) string {
		l := loadIR(t, filepath.Join(testutil.Testdata(), "..", "scc"), "./...")
		opts := DefaultOptions()
		opts.Workers = workers
		e := l.engine(l.sites(rules), rules, opts)
		e.SetIterationDump(dir)
		e.Run()
		var b bytes.Buffer
		if err := e.WriteSummaryDump(&b); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	readAll := func(dir string) map[string]string {
		files, err := filepath.Glob(filepath.Join(dir, "scc*-iter*.tsv"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no files in %s (%v)", dir, err)
		}
		out := map[string]string{}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			out[filepath.Base(f)] = string(b)
		}
		return out
	}
	d1, d2 := t.TempDir(), t.TempDir()
	final1 := run(d1, 1)
	final2 := run(d2, 3)
	if final1 != final2 {
		t.Errorf("the final summaries depend on the number of workers")
	}
	a, b := readAll(d1), readAll(d2)
	if len(a) != len(b) {
		t.Fatalf("%d files with one worker, %d with three", len(a), len(b))
	}
	for name, text := range a {
		if b[name] != text {
			t.Errorf("%s differs between one and three workers", name)
		}
	}
	if _, ok := a["scc0-iter1.tsv"]; !ok {
		t.Fatalf("no scc0-iter1.tsv: %v", a)
	}
	for name, text := range a {
		lines := strings.Split(strings.TrimSpace(text), "\n")
		if !strings.HasPrefix(lines[0], "# pass=") {
			t.Errorf("%s: first line %q", name, lines[0])
		}
		prev := ""
		if !strings.HasPrefix(lines[1], "# outkeys ") {
			t.Errorf("%s: second line %q", name, lines[1])
		}
		for _, l := range lines[2:] {
			if strings.HasPrefix(l, "# outpath ") {
				continue
			}
			f := strings.Split(l, "\t")
			if len(f) != 18 || !strings.HasPrefix(f[13], "outdepth=") || !strings.HasPrefix(f[14], "outidx=") || !strings.HasPrefix(f[15], "maxout=") || !strings.HasPrefix(f[1], "elems=") || !strings.HasPrefix(f[2], "steps=") || len(f[3]) != 64 ||
				!strings.HasPrefix(f[4], "outs=") || !strings.HasPrefix(f[11], "depth=") || !strings.HasPrefix(f[16], "chg=") {
				t.Errorf("%s: a line of the wrong form (%d fields): %q", name, len(f), l)
			}
			if prev >= f[0] {
				t.Errorf("%s: not in the order of the IDs: %q before %q", name, prev, f[0])
			}
			prev = f[0]
		}
	}
	// without the dump the summaries are the same
	l := loadIR(t, filepath.Join(testutil.Testdata(), "..", "scc"), "./...")
	e := l.engine(l.sites(rules), rules, DefaultOptions())
	e.Run()
	var plain bytes.Buffer
	if err := e.WriteSummaryDump(&plain); err != nil {
		t.Fatal(err)
	}
	if plain.String() != final1 {
		t.Errorf("the summaries change when the rounds are written")
	}
}
