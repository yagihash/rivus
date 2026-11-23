package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// deadAllowCount answers: of the rows that the zoo does not hit (`internal/model/builtin/*.dead-allow`), how many were judged by a differential
// test, how many were in a package that was run and were not judged, and how many are in a package that no report has. The reports are the
// `report*.json` files under `dir` (what `-run-changed` writes: run it with the empty tree as the base to run every package). Go only: the
// rows of the other languages are counted as having no report.
func deadAllowCount(repo, dir string) error {
	judged := map[string]bool{}
	notJudged := map[string]string{}
	ran := map[string]bool{}
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
		for _, r := range rep.Rows {
			k := normalMajor(r.Symbol)
			if r.Judged {
				judged[k] = true
			} else if _, ok := notJudged[k]; !ok {
				notJudged[k] = r.Reason
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	files, _ := filepath.Glob(filepath.Join(repo, "internal", "model", "builtin", "*.dead-allow"))
	fmt.Printf("%-28s %6s %8s %12s %11s\n", "file", "rows", "judged", "not judged", "no report")
	var tot [4]int
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var n [4]int
		for _, l := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(l, "#") || strings.TrimSpace(l) == "" {
				continue
			}
			sym := normalMajor(strings.TrimSpace(strings.SplitN(l, " | ", 2)[0]))
			n[0]++
			lang := strings.SplitN(filepath.Base(f), "-", 2)[0]
			switch {
			case judged[sym]:
				n[1]++
			case lang == "go" && ran[SymbolPackage("go", sym)]:
				n[2]++
			default:
				n[3]++
			}
		}
		fmt.Printf("%-28s %6d %8d %12d %11d\n", strings.TrimSuffix(filepath.Base(f), ".dead-allow"), n[0], n[1], n[2], n[3])
		for i := range tot {
			tot[i] += n[i]
		}
	}
	fmt.Printf("%-28s %6d %8d %12d %11d\n", "total", tot[0], tot[1], tot[2], tot[3])
	return nil
}

var majorRe = regexp.MustCompile(`/v[0-9]+(/|\.|$)`)

// normalMajor writes the major of a module path as /v*, so that the symbols of a report (one major) and of a row (any major) are the same.
func normalMajor(s string) string {
	return majorRe.ReplaceAllString(s, "/v*$1")
}
