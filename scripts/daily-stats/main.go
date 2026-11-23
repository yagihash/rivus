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
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// orphan is a remote branch with commits that main does not have.
type orphan struct {
	Name    string
	Commits int
}

// classifyBranches splits the remote branches (name -> commits not in main) into the orphans (commits not in main and
// no open pull request, sorted by name) and the count of the renovate/ ones (reported apart, not hidden). main and HEAD are skipped.
func classifyBranches(ahead map[string]int, open map[string]bool) (orphans []orphan, renovate int) {
	names := make([]string, 0, len(ahead))
	for n := range ahead {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if n == "main" || n == "HEAD" || ahead[n] == 0 || open[n] {
			continue
		}
		if strings.HasPrefix(n, "renovate/") {
			renovate++
			continue
		}
		orphans = append(orphans, orphan{n, ahead[n]})
	}
	return orphans, renovate
}

// orphanReport prints the orphan lines from git and gh. A failure is a warning: the line is skipped, not the whole report.
func orphanReport(ref string) {
	refs, err := git("for-each-ref", "--format=%(refname:strip=3)", "refs/remotes/origin")
	if err != nil {
		fmt.Fprintln(os.Stderr, "orphan remote branches: skipped (git for-each-ref):", err)
		return
	}
	heads, err := exec.Command("gh", "pr", "list", "--state", "open", "--limit", "1000", "--json", "headRefName", "--jq", ".[].headRefName").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orphan remote branches: skipped (gh pr list):", err)
		return
	}
	open := map[string]bool{}
	for _, h := range strings.Fields(string(heads)) {
		open[h] = true
	}
	ahead := map[string]int{}
	for _, n := range strings.Fields(refs) {
		if n == "main" || n == "HEAD" {
			continue
		}
		c, err := git("rev-list", "--count", ref+"..origin/"+n)
		if err != nil {
			continue
		}
		ahead[n], _ = strconv.Atoi(strings.TrimSpace(c))
	}
	orphans, renovate := classifyBranches(ahead, open)
	fmt.Printf("orphan remote branches: %d\n", len(orphans))
	for _, o := range orphans {
		fmt.Printf("  %s (%d commits)\n", o.Name, o.Commits)
	}
	fmt.Printf("renovate branches: %d\n", renovate)
}

// parityTodoRe finds a `@todo(<cell>: PARITY: ...)` on a line: the cell that another language has and this one has not yet.
var parityTodoRe = regexp.MustCompile(`@todo\(([^:)]*)[^\n]*PARITY`)

// countParityTodos counts the cells of the lines that have a `@todo(` with PARITY after it (the same cell on more than one
// line is one), in the sources of the zoo of a language: bench/zoo-<lang>/ and the packages bench/zoo-<lang>-pkg/<package>/.
func countParityTodos(bench, lang, ext string) int {
	cells := map[string]bool{}
	for _, dir := range []string{"zoo-" + lang, "zoo-" + lang + "-pkg"} {
		_ = filepath.Walk(filepath.Join(bench, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ext) {
				return nil
			}
			b, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			for _, m := range parityTodoRe.FindAllStringSubmatch(string(b), -1) {
				cells[strings.TrimSpace(m[1])] = true
			}
			return nil
		})
	}
	return len(cells)
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
	fmt.Printf("zoo PARITY todo: rust %d, dart %d, go %d\n", countParityTodos("bench", "rust", ".rs"), countParityTodos("bench", "dart", ".dart"), countParityTodos("bench", "go", ".go"))
	orphanReport(*ref)
}
