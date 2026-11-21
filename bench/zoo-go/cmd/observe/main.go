//go:build zooexec

// Command observe runs every case of the zoo once and writes what the sinks got. It is built
// with -tags zooexec (and -cover); cmd/rivus TestZooExec runs it three times (variant 0, variant 1,
// variant 0 in the reverse order) and checks them. It is not part of what rivus analyses.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"zoo/mark"
)

func main() {
	reverse := flag.Bool("reverse", false, "run the cases in the reverse order")
	out := flag.String("out", "", "the file to write")
	flag.Parse()
	hits := mark.Run(*reverse)
	b, err := json.MarshalIndent(map[string]any{"variant": mark.Variant, "reverse": *reverse, "cases": mark.CaseIDs(), "hits": hits}, "", " ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
