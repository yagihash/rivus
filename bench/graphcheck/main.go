// graphcheck compares what two versions of rivus built from the same code: the IR, the edges of the call graph with
// their candidates, and the summary of every function. The same means the representation did not change; it is what a
// change for memory or cost has to show (docs/WORKFLOW.md). bench/compare.sh --graph-check runs it.
//
//	graphcheck ir        A.jsonl B.jsonl   the output of `rivus debug extract -all` (the comparison of the acceptance test: accept.Compare)
//	graphcheck edges     A.db B.db         the graph files of `rivus index` (one line for each edge: the call site, the callee, kind, ncand)
//	graphcheck summaries A.txt B.txt       the files of RIVUS_INTERNAL_SUMMARY_DUMP (one line for each function)
//
// It prints one line: "<what>: same (<n> ...)", or "<what>: DIFFERENT (...)" and the first lines of each kind of
// difference. The exit code is 0 for the same, 1 for different, 2 for a file that cannot be read.
package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/yagihash/rivus/internal/frontend/accept"
	"github.com/yagihash/rivus/internal/ir"
)

const listLimit = 5 // how many of each kind are listed

func main() { os.Exit(realMain(os.Args[1:])) }

func realMain(args []string) int {
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: graphcheck ir|edges|summaries A B")
		return 2
	}
	var out string
	var same bool
	var err error
	switch args[0] {
	case "ir":
		out, same, err = compareIR(args[1], args[2])
	case "edges":
		out, same, err = compareEdges(args[1], args[2])
	case "summaries":
		out, same, err = compareLines("summaries", "functions", args[1], args[2], summaryKey)
	default:
		fmt.Fprintf(os.Stderr, "graphcheck: unknown comparison %q\n", args[0])
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "graphcheck: %s: %v\n", args[0], err)
		return 2
	}
	fmt.Print(out)
	if !same {
		return 1
	}
	return 0
}

func readIR(path string) (*ir.Program, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ir.Read(bufio.NewReaderSize(f, 1<<20))
}

func compareIR(a, b string) (string, bool, error) {
	pa, err := readIR(a)
	if err != nil {
		return "", false, err
	}
	pb, err := readIR(b)
	if err != nil {
		return "", false, err
	}
	r := accept.Compare(pa, pb)
	if r.Empty() {
		return fmt.Sprintf("IR: same (%d functions)\n", len(pa.Funcs)), true, nil
	}
	return "IR: DIFFERENT\n" + r.Summary(listLimit), false, nil
}

// edgeLines is one line for each edge of a graph file, sorted: the function the call is in, where, the function it may
// reach, the kind, and the number of candidates of the call site (the form of the acceptance test).
func edgeLines(path string) ([]string, error) {
	if _, err := os.Stat(path); err != nil {
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
	var out []string
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

func compareEdges(a, b string) (string, bool, error) {
	la, err := edgeLines(a)
	if err != nil {
		return "", false, err
	}
	lb, err := edgeLines(b)
	if err != nil {
		return "", false, err
	}
	if equalLines(la, lb) {
		return fmt.Sprintf("edges: same (%d edges with their candidates)\n", len(la)), true, nil
	}
	onlyA, onlyB := multisetDiff(la, lb)
	var sb strings.Builder
	fmt.Fprintf(&sb, "edges: DIFFERENT (%d edges in the first, %d in the second; only in the first: %d, only in the second: %d)\n", len(la), len(lb), len(onlyA), len(onlyB))
	list(&sb, "only in the first", onlyA)
	list(&sb, "only in the second", onlyB)
	return sb.String(), false, nil
}

// summaryKey is the function ID of a line of the dump (the first field).
func summaryKey(line string) string { return strings.SplitN(line, "\t", 2)[0] }

// compareLines compares two files of lines, each line with a key (a function ID): what only one side has, and the keys whose line differs.
func compareLines(what, unit, a, b string, key func(string) string) (string, bool, error) {
	ma, err := readKeyed(a, key)
	if err != nil {
		return "", false, err
	}
	mb, err := readKeyed(b, key)
	if err != nil {
		return "", false, err
	}
	var onlyA, onlyB, differ []string
	for k, v := range ma {
		w, ok := mb[k]
		switch {
		case !ok:
			onlyA = append(onlyA, k)
		case v != w:
			differ = append(differ, k)
		}
	}
	for k := range mb {
		if _, ok := ma[k]; !ok {
			onlyB = append(onlyB, k)
		}
	}
	if len(onlyA)+len(onlyB)+len(differ) == 0 {
		return fmt.Sprintf("%s: same (%d %s)\n", what, len(ma), unit), true, nil
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	sort.Strings(differ)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: DIFFERENT (%s only in the first: %d, only in the second: %d, with a different summary: %d)\n", what, unit, len(onlyA), len(onlyB), len(differ))
	list(&sb, "only in the first", onlyA)
	list(&sb, "only in the second", onlyB)
	list(&sb, "different", differ)
	return sb.String(), false, nil
}

func readKeyed(path string, key func(string) string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			m[key(line)] = line
		}
	}
	return m, sc.Err()
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// multisetDiff takes two sorted lists and returns what each has more than the other.
func multisetDiff(a, b []string) (onlyA, onlyB []string) {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			i++
			j++
		case a[i] < b[j]:
			onlyA = append(onlyA, a[i])
			i++
		default:
			onlyB = append(onlyB, b[j])
			j++
		}
	}
	onlyA = append(onlyA, a[i:]...)
	onlyB = append(onlyB, b[j:]...)
	return
}

func list(sb *strings.Builder, what string, items []string) {
	for i, s := range items {
		if i == listLimit {
			fmt.Fprintf(sb, "  %s: ... and %d more\n", what, len(items)-listLimit)
			return
		}
		fmt.Fprintf(sb, "  %s: %s\n", what, s)
	}
}
