package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/accept"
	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
)

// The analysis with one extractor gives the same result as the analysis with another: the same findings, the same
// uncertainties, the same counts in the summary and in the stats, the same edges of the calls. There is no list of
// allowed differences. The extractor in use is read twice here; a new one is put in as the second.

// machineSummaryKeys are the fields of the summary that the machine and its load decide, as the times are: the memory the
// run used and the share of the collector (#359). A test that compares a whole summary leaves them out, with this list.
var machineSummaryKeys = []string{"memory_peak_bytes", "memory_limit_exceeded", "gc_cpu_fraction", "live_heap_max_bytes"}

// machineStatsKeys are the clocks in the stats.
var machineStatsKeys = []string{"duration_ms", "dep_spent_ms", "own_spent_ms"}

// acceptResult is what an analysis gave, by item. Each item is the JSON of it with the keys in order.
type acceptResult struct {
	items map[string]string
}

var acceptItems = []string{"findings", "uncertainties", "summary", "stats", "edges"}

// acceptRun analyses the Go code in dir with the extractor (nil: the one the analysis has by itself) and returns the
// items that are compared. args are the flags of `rivus analyze`.
func acceptRun(t *testing.T, ex *accept.Extractor, dir string, args []string, patterns ...string) acceptResult {
	t.Helper()
	return acceptRunIn(t, ex, false, dir, args, patterns...)
}

// acceptRunIn is acceptRun with the IR in memory (spill false) or in the file (true: ADR-123).
func acceptRunIn(t *testing.T, ex *accept.Extractor, spill bool, dir string, args []string, patterns ...string) acceptResult {
	t.Helper()
	fs := flag.NewFlagSet("accept", flag.ContinueOnError)
	var o analyzeOpts
	o.register(fs)
	if err := fs.Parse(append([]string{"-C", dir, "-pass-stats"}, args...)); err != nil {
		t.Fatal(err)
	}
	o.extractor = ex
	o.irSpill = spill
	var edges []string
	o.afterRead = func(l *loaded, r *model.Rules, st *sites.Result, g *sg.Program, _ map[string]string) error {
		if spill {
			if l.disk == nil {
				t.Error("the IR is not in the file")
			} else if st := l.disk.Stats(); st.Store.Funcs == 0 || st.Store.Bytes == 0 || st.Reads < st.Store.Funcs/2 {
				t.Errorf("the file was not used: %+v", st)
			} else {
				t.Logf("in the file: %d functions, %d bytes, %d calls in the table of candidates, %d reads, %d from the cache", st.Store.Funcs, st.Store.Bytes, st.Updates, st.Reads, st.CacheHits)
			}
		}
		var err error
		edges, err = acceptEdges(filepath.Join(t.TempDir(), "graph.db"), g, st)
		return err
	}
	rep, err := o.analyze(patterns)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	m["edges"] = edges
	return acceptItemsOf(t, m)
}

// acceptItemsOf cuts the items out of a report (as a map), without the fields that the machine decides.
func acceptItemsOf(t *testing.T, m map[string]any) acceptResult {
	t.Helper()
	out := acceptResult{items: map[string]string{}}
	if st, ok := m["stats"].(map[string]any); ok {
		for _, k := range machineStatsKeys {
			delete(st, k)
		}
		// packages_top (-pass-stats) has the time each package took
		if pk, ok := st["packages_top"].([]any); ok {
			for _, row := range pk {
				if r, ok := row.(map[string]any); ok {
					delete(r, "spent_ms")
				}
			}
		}
	}
	if sm, ok := m["summary"].(map[string]any); ok {
		for _, k := range machineSummaryKeys {
			delete(sm, k)
		}
	}
	for _, k := range acceptItems {
		b, err := json.Marshal(m[k]) // the keys of a map are in order
		if err != nil {
			t.Fatal(err)
		}
		out.items[k] = string(b)
	}
	return out
}

// acceptEdges lists the edges of the call graph as `rivus index` writes them: one line for each edge, the function the
// call is in, where it is, the function it may reach, and the kind.
func acceptEdges(path string, g *sg.Program, st *sites.Result) ([]string, error) {
	if _, err := graphdb.Write(path, &graphdb.Input{Graph: g, Sites: st.Sites}); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT f.fid, coalesce(fl.path, ''), c.line, c.col, t.fid, e.kind, e.ncand
		FROM edges e JOIN callsites c ON c.id = e.callsite_id JOIN funcs f ON f.id = e.from_func JOIN funcs t ON t.id = e.to_func
		LEFT JOIN files fl ON fl.id = c.file_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var from, file, to, kind string
		var line, col, ncand int
		if err := rows.Scan(&from, &file, &line, &col, &to, &kind, &ncand); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("%s %s:%d:%d -> %s %s %d", from, file, line, col, to, kind, ncand))
	}
	sort.Strings(out)
	return out, rows.Err()
}

// acceptDiff says what differs between two results: how many items, and the first lines of each that do.
func acceptDiff(a, b acceptResult) []string {
	var out []string
	for _, k := range acceptItems {
		if a.items[k] == b.items[k] {
			continue
		}
		var x, y any
		json.Unmarshal([]byte(a.items[k]), &x)
		json.Unmarshal([]byte(b.items[k]), &y)
		out = append(out, fmt.Sprintf("%s differs:\n%s", k, firstDifferences(k, x, y, 5)))
	}
	return out
}

// firstDifferences lists at most n places where two JSON values differ, with the path to each.
func firstDifferences(path string, a, b any, n int) string {
	var out []string
	var walk func(path string, a, b any)
	walk = func(path string, a, b any) {
		if len(out) >= n || reflect.DeepEqual(a, b) {
			return
		}
		switch x := a.(type) {
		case map[string]any:
			if y, ok := b.(map[string]any); ok {
				keys := map[string]bool{}
				for k := range x {
					keys[k] = true
				}
				for k := range y {
					keys[k] = true
				}
				sorted := make([]string, 0, len(keys))
				for k := range keys {
					sorted = append(sorted, k)
				}
				sort.Strings(sorted)
				for _, k := range sorted {
					walk(path+"."+k, x[k], y[k])
				}
				return
			}
		case []any:
			if y, ok := b.([]any); ok && len(x) == len(y) {
				for i := range x {
					walk(fmt.Sprintf("%s[%d]", path, i), x[i], y[i])
				}
				return
			}
			if y, ok := b.([]any); ok {
				out = append(out, fmt.Sprintf("  %s: %d elements and %d elements", path, len(x), len(y)))
				return
			}
		}
		ja, _ := json.Marshal(a)
		jb, _ := json.Marshal(b)
		out = append(out, fmt.Sprintf("  %s: %s != %s", path, clip(string(ja)), clip(string(jb))))
	}
	walk(path, a, b)
	return strings.Join(out, "\n")
}

func clip(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

// acceptBoth analyses dir with the two extractors (the second is the new one) and with the analysis by itself, and fails
// when anything differs. The run without an extractor tells that the way the extractor is put in does not change the result.
func acceptBoth(t *testing.T, dir string, args []string, patterns ...string) {
	t.Helper()
	by := acceptRun(t, nil, dir, args, patterns...)
	// an empty result is the same as another empty one: the comparison has to be on something
	for _, k := range []string{"findings", "edges"} {
		if by.items[k] == "[]" || by.items[k] == "null" {
			t.Fatalf("the analysis by itself gave no %s: nothing is compared", k)
		}
	}
	for _, ex := range []accept.Extractor{accept.Current, accept.Current} {
		ex := ex
		got := acceptRun(t, &ex, dir, args, patterns...)
		if d := acceptDiff(by, got); len(d) != 0 {
			t.Fatalf("the analysis with %s differs from the analysis by itself:\n%s", ex.Name, strings.Join(d, "\n"))
		}
	}
}

func TestAcceptZoo(t *testing.T) {
	dir := zooRoot(t)
	var args []string
	for _, d := range zooGoDepDirs {
		args = append(args, "-dep-dir", d)
	}
	acceptBoth(t, dir, append(args, "-rules", filepath.Join(dir, "rules.yaml")), "./cells/...")
}

func TestAcceptCurated(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	acceptBoth(t, dir, []string{"-rules", filepath.Join(dir, "rules.yaml")}, "./...")
}

// The comparison sees a difference: a result with one finding left out is not the same.
func TestAcceptSeesAMissingFinding(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	ex := accept.Current
	a := acceptRun(t, &ex, dir, []string{"-rules", filepath.Join(dir, "rules.yaml")}, "./...")
	var fs []any
	if err := json.Unmarshal([]byte(a.items["findings"]), &fs); err != nil || len(fs) < 2 {
		t.Fatalf("findings: %v, %d", err, len(fs))
	}
	b := acceptResult{items: map[string]string{}}
	for k, v := range a.items {
		b.items[k] = v
	}
	cut, _ := json.Marshal(fs[1:])
	b.items["findings"] = string(cut)
	if d := acceptDiff(a, b); len(d) != 1 || !strings.HasPrefix(d[0], "findings differs") {
		t.Fatalf("one finding less is not seen: %q", d)
	}
	// a difference in an edge, an uncertainty and the stats is seen too
	for _, k := range []string{"edges", "uncertainties", "stats", "summary"} {
		c := acceptResult{items: map[string]string{}}
		for kk, v := range a.items {
			c.items[kk] = v
		}
		c.items[k] = a.items[k] + "x"
		if len(acceptDiff(a, c)) != 1 {
			t.Errorf("a difference in %s is not seen", k)
		}
	}
	if len(acceptDiff(a, a)) != 0 {
		t.Error("the same result differs from itself")
	}
}

// The IR in a file gives the same result as the IR in memory: the same findings, uncertainties, summary, stats and
// edges. It is read two ways: the functions go to the file as the extractor converts them (no extractor put in), and the
// IR that an extractor made in memory is put into the file (an extractor put in). There is no list of allowed differences.
func acceptSpillBoth(t *testing.T, dir string, args []string, patterns ...string) {
	t.Helper()
	mem := acceptRunIn(t, nil, false, dir, args, patterns...)
	for _, k := range []string{"findings", "edges"} {
		if mem.items[k] == "[]" || mem.items[k] == "null" {
			t.Fatalf("the analysis in memory gave no %s: nothing is compared", k)
		}
	}
	ex := accept.Current
	for name, got := range map[string]acceptResult{
		"converted into the file": acceptRunIn(t, nil, true, dir, args, patterns...),
		"put into the file":       acceptRunIn(t, &ex, true, dir, args, patterns...),
	} {
		if d := acceptDiff(mem, got); len(d) != 0 {
			t.Fatalf("the analysis with the IR %s differs from the one in memory:\n%s", name, strings.Join(d, "\n"))
		}
	}
}

func zooArgs(t *testing.T) (string, []string) {
	dir := zooRoot(t)
	var args []string
	for _, d := range zooGoDepDirs {
		args = append(args, "-dep-dir", d)
	}
	return dir, append(args, "-rules", filepath.Join(dir, "rules.yaml"))
}

func TestAcceptSpillZoo(t *testing.T) {
	dir, args := zooArgs(t)
	acceptSpillBoth(t, dir, args, "./cells/...")
}

func TestAcceptSpillCurated(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	acceptSpillBoth(t, dir, []string{"-rules", filepath.Join(dir, "rules.yaml")}, "./...")
}

// The order of the load (RIVUS_INTERNAL_LOAD_ORDER) changes nothing with the IR in the file: each order gives the
// result that the IR in memory gives for the same order.
func TestAcceptSpillLoadOrder(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-rules", filepath.Join(dir, "rules.yaml")}
	base := acceptRunIn(t, nil, false, dir, args, "./...")
	for _, seed := range []string{"a", "b"} {
		t.Setenv("RIVUS_INTERNAL_LOAD_ORDER", seed)
		for _, spill := range []bool{false, true} {
			got := acceptRunIn(t, nil, spill, dir, args, "./...")
			if d := acceptDiff(base, got); len(d) != 0 {
				t.Fatalf("order %q, IR in the file %v: differs from the first run:\n%s", seed, spill, strings.Join(d, "\n"))
			}
		}
	}
}

// The calls of function values get their candidates from valuecalls, which writes them on the function that holds the
// call: with the IR in the file, that function is held in memory so that the write is there for the graph. A program
// that has such calls (the function is passed, and called through the parameter and through a variable) gives the same
// result in the file as in memory, and the file holds those functions.
func TestAcceptSpillFunctionValues(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/fv\n\ngo 1.22\n")
	write("main.go", `package main

import (
	"net/http"
	"os/exec"
)

func run(f func(string), s string) { f(s) }

func sink(s string) { exec.Command("sh", "-c", s).Run() }

func quiet(s string) {}

func pick(b bool) func(string) {
	if b {
		return sink
	}
	return quiet
}

func h(w http.ResponseWriter, r *http.Request) {
	run(sink, r.FormValue("x"))
	g := sink
	g(r.FormValue("y"))
	pick(r.FormValue("z") == "")(r.FormValue("w"))
}

func h2(w http.ResponseWriter, r *http.Request) {
	// the function that is called is the one that was passed: what is passed to the other is not the flow
	run(quiet, r.FormValue("a"))
	run(sink, "constant")
}

func main() { http.HandleFunc("/", h); http.HandleFunc("/2", h2); http.ListenAndServe(":0", nil) }
`)
	mem := acceptRunIn(t, nil, false, dir, nil, "./...")
	if mem.items["findings"] == "[]" || mem.items["findings"] == "null" {
		t.Fatalf("no finding in memory: nothing is compared: %s", mem.items["uncertainties"])
	}
	got := acceptRunIn(t, nil, true, dir, nil, "./...")
	if d := acceptDiff(mem, got); len(d) != 0 {
		t.Fatalf("the IR in the file differs from the one in memory:\n%s", strings.Join(d, "\n"))
	}
}
