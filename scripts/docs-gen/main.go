// docs-gen prints the table of the issues, made from the headers of docs/issues/*.md (`make docs-gen`).
// With -o <file> it writes the table there instead (docs/ISSUES-TABLE.md is ignored by git).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yagihash/rivus/internal/docsgen"
)

func main() {
	dir := flag.String("dir", "docs/issues", "the directory of the issue files")
	out := flag.String("o", "", "write here instead of stdout")
	flag.Parse()
	issues, errs := docsgen.Load(*dir)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "docs-gen:", e)
	}
	t := docsgen.Table(issues)
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
