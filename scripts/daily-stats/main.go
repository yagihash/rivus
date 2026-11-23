// daily-stats counts the pull requests merged into main since a date, by what they change: `make daily-stats SINCE=2026-11-20`.
// A merge is ENGINE/MODEL/EXTRACTOR when it changes a file of the analysis (engine, model, extractor, or the code of cmd/rivus
// that is not a test), and TOOLS/CI/DOCS otherwise. It reads `git log --first-parent --merges` and, for each merge,
// `git diff --name-only <merge>^1 <merge>`. (docs/PRINCIPLES.md, section 7.)
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// enginePrefixes are the directories whose files are work on the analysis itself.
var enginePrefixes = []string{
	"internal/taint/", "internal/sg/", "internal/irssa/", "internal/sites/", "internal/callgraph/",
	"internal/frontend/", "internal/model/", "extractor/",
}

// isEngineFile reports whether a changed file is ENGINE/MODEL/EXTRACTOR work.
func isEngineFile(path string) bool {
	for _, p := range enginePrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return strings.HasPrefix(path, "cmd/rivus/") && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

// isEngineChange reports whether a change (the list of its files) is ENGINE/MODEL/EXTRACTOR work: one such file is enough.
func isEngineChange(files []string) bool {
	for _, f := range files {
		if isEngineFile(f) {
			return true
		}
	}
	return false
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return string(out), err
}

func main() {
	since := flag.String("since", "", "count the merges since this date (git --since: 2026-11-20, \"3 days ago\")")
	ref := flag.String("ref", "origin/main", "the branch to read")
	flag.Parse()
	if *since == "" {
		fmt.Fprintln(os.Stderr, "usage: daily-stats -since <date> [-ref origin/main]")
		os.Exit(2)
	}
	log, err := git("log", "--first-parent", "--merges", "--since="+*since, "--format=%H", *ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, "git log:", err)
		os.Exit(1)
	}
	engine, other := 0, 0
	for _, h := range strings.Fields(log) {
		names, err := git("diff", "--name-only", h+"^1", h)
		if err != nil {
			fmt.Fprintln(os.Stderr, "git diff", h, err)
			os.Exit(1)
		}
		if isEngineChange(strings.Fields(names)) {
			engine++
		} else {
			other++
		}
	}
	fmt.Printf("daily-stats since %s (%s): ENGINE/MODEL/EXTRACTOR %d, TOOLS/CI/DOCS %d\n", *since, *ref, engine, other)
}
