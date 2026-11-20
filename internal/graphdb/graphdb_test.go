package graphdb_test

import (
	"os"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/graphdb"
)

// RIVUS_GRAPH_FILE names a graph file of a large program: the time to open it, and to find a call by its position.
func TestOpenAndLookupTimes(t *testing.T) {
	path := os.Getenv("RIVUS_GRAPH_FILE")
	if path == "" {
		t.Skip("RIVUS_GRAPH_FILE is not set")
	}
	start := time.Now()
	d, err := graphdb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	t.Logf("open: %v", time.Since(start))
	start = time.Now()
	info, err := d.Info()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("info (counts): %v %+v", time.Since(start), info.Counts)

	// a call in the middle of the table
	var file string
	var line, col int
	rows, err := d.QueryForTest(`SELECT fl.path, c.line, c.col FROM callsites c JOIN files fl ON fl.id = c.file_id WHERE c.kind = 'interface' LIMIT 1 OFFSET 500`)
	if err != nil {
		t.Fatal(err)
	}
	file, line, col = rows.File, rows.Line, rows.Col
	for _, c := range []int{col, 0} {
		start = time.Now()
		got, err := d.CallSitesAt(file, line, c)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s:%d:%d -> %d call sites, %d candidates in the first: %v", file, line, c, len(got), len(got[0].Targets), time.Since(start))
	}
	start = time.Now()
	fs, err := d.FuncBySymbol("fmt.Sprintf")
	t.Logf("symbol fmt.Sprintf: %d functions, %v (%v)", len(fs), time.Since(start), err)
}
