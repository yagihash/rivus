package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// deadAllowCount prints two tables of the reports under `dir`: the rows that the differential test judged and did not judge, per group of
// packages (the Go standard library, and each other package: the numbers of bench/LEDGER.md), and then the answer to this: of the rows that the zoo does not hit (`internal/model/builtin/*.dead-allow`), how many were judged by a differential
// test, how many were in a package that was run and were not judged, and how many are in a package that no report has. The reports are the
// `report*.json` files under `dir` (what `-run-changed` writes: run it with the empty tree as the base to run every package). Go only: the
// rows of the other languages are counted as having no report.
func deadAllowCount(repo, dir string) error {
	judged := map[string]bool{}
	failing := map[string]bool{} // the rows that were judged and have a MISS or an EXTRA
	notJudged := map[string]string{}
	ran := map[string]bool{}
	type tally struct{ pkgs, rows, judged, not, miss, extra, errs int }
	groups := map[string]*tally{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "report") || !strings.HasSuffix(d.Name(), ".json") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var rep Report
		if json.Unmarshal(data, &rep) != nil {
			return nil
		}
		ran[normalMajor(rep.Package)] = true
		g := rep.Language + " " + normalMajor(rep.Package)
		if rep.Language == "go" && !strings.Contains(strings.SplitN(rep.Package, "/", 2)[0], ".") {
			g = "go standard library"
		}
		t := groups[g]
		if t == nil {
			t = &tally{}
			groups[g] = t
		}
		t.pkgs++
		t.rows += rep.Counts["rows_judged"] + rep.Counts["rows_not_judged"]
		t.judged += rep.Counts["rows_judged"]
		t.not += rep.Counts["rows_not_judged"]
		t.miss += rep.Counts["MISS"]
		t.extra += rep.Counts["EXTRA"]
		t.errs += len(rep.Errors)
		for _, r := range rep.Rows {
			k := normalMajor(r.Symbol)
			if r.Judged {
				judged[k] = true
				failing[k] = failing[k] || r.Failed
			} else if _, ok := notJudged[k]; !ok {
				notJudged[k] = r.Reason
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	var names []string
	for g := range groups {
		names = append(names, g)
	}
	sort.Strings(names)
	fmt.Printf("%-60s %5s %6s %7s %11s %5s %6s %6s\n", "group", "pkgs", "rows", "judged", "not judged", "MISS", "EXTRA", "errors")
	for _, g := range names {
		t := groups[g]
		fmt.Printf("%-60s %5d %6d %7d %11d %5d %6d %6d\n", g, t.pkgs, t.rows, t.judged, t.not, t.miss, t.extra, t.errs)
	}
	fmt.Println()
	files, _ := filepath.Glob(filepath.Join(repo, "internal", "model", "builtin", "*.dead-allow"))
	fmt.Printf("%-28s %6s %10s %14s %12s %11s\n", "file", "rows", "confirmed", "judged, wrong", "not judged", "no report")
	var tot [5]int
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var n [5]int
		for _, l := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(l, "#") || strings.TrimSpace(l) == "" {
				continue
			}
			sym := normalMajor(strings.TrimSpace(strings.SplitN(l, " | ", 2)[0]))
			n[0]++
			lang := strings.SplitN(filepath.Base(f), "-", 2)[0]
			switch {
			case judged[sym] && !failing[sym]:
				n[1]++
			case judged[sym]:
				n[2]++
			case lang == "go" && ran[SymbolPackage("go", sym)]:
				n[3]++
			default:
				n[4]++
			}
		}
		fmt.Printf("%-28s %6d %10d %14d %12d %11d\n", strings.TrimSuffix(filepath.Base(f), ".dead-allow"), n[0], n[1], n[2], n[3], n[4])
		for i := range tot {
			tot[i] += n[i]
		}
	}
	fmt.Printf("%-28s %6d %10d %14d %12d %11d\n", "total", tot[0], tot[1], tot[2], tot[3], tot[4])
	return nil
}

var majorRe = regexp.MustCompile(`/v[0-9]+(/|\.|$)`)

// normalMajor writes the major of a module path as /v*, so that the symbols of a report (one major) and of a row (any major) are the same.
func normalMajor(s string) string {
	return majorRe.ReplaceAllString(s, "/v*$1")
}
