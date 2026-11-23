// docs-gen prints the tables made from the files of docs/issues (the issues) and docs/adr (the ADRs): `make docs-gen`.
// With the argument `issues` or `adr` it prints that one only (default: both). With -o <file> it writes there instead
// (docs/ISSUES-TABLE.md, docs/ADR-TABLE.md and docs/TABLE.md are ignored by git).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yagihash/rivus/internal/docsgen"
)

func main() {
	issuesDir := flag.String("dir", "docs/issues", "the directory of the issue files")
	adrDir := flag.String("adr", "docs/adr", "the directory of the ADR files")
	out := flag.String("o", "", "write here instead of stdout")
	flag.Parse()
	what := flag.Arg(0)
	if what != "" && what != "issues" && what != "adr" || flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: docs-gen [-o file] [issues|adr]")
		os.Exit(2)
	}
	var errs []error
	t := ""
	if what == "" || what == "issues" {
		issues, e := docsgen.Load(*issuesDir)
		errs = append(errs, e...)
		t += docsgen.Table(issues)
	}
	if what == "" || what == "adr" {
		adrs, e := docsgen.LoadAdr(*adrDir)
		errs = append(errs, e...)
		if t != "" {
			t += "\n"
		}
		t += docsgen.AdrTable(adrs)
	}
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "docs-gen:", e)
	}
	if *out == "" {
		fmt.Print(t)
	} else if err := os.WriteFile(*out, []byte(t), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "docs-gen:", err)
		os.Exit(1)
	}
	if len(errs) > 0 {
		os.Exit(1)
	}
}
