package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The marks that loosen the truth of the zoo (docs/ROADMAP.md, section 1): @miss, @fp, @noisy, @na, @todo, @manual
// and @sink(none) of the three zoos are listed in bench/zoo/marks.lock, one row for a mark of a line:
//
//	<lang> TAB <file> TAB <kind> TAB <the annotations of the line> TAB <the header of the case the line is in>
//
// (no line number, so that an edit above does not change the rows; the last column is the line of the @vuln /
// @fixed / @decoy that comes before it, or "-", so that a mark that is moved to another case is a new row). TestZooMarksLock holds the file to the sources;
// scripts/zoo-loosened-check.sh fails a PR whose rows (against the merge base) loosen the zoo without a "Zoo-Loosened:"
// line in a commit message (a row added to a cell that the lock has, a row of a new cell of a kind other than @sink(none),
// @miss and @fp), and a new @miss or @fp row that names no issue. A row that goes away (a mark that is fixed) needs nothing. To write the file again: ZOO_MARKS_UPDATE=1.

const zooMarksLockPath = "bench/zoo/marks.lock"

var (
	zooMarksRoleRe  = regexp.MustCompile(`@(vuln|fixed|decoy)\b`)
	zooMarksTokenRe = regexp.MustCompile(`@[a-z]+(?:\([^)]*\))?`)
	zooMarksKinds   = map[string]string{"@miss": "miss", "@fp": "fp", "@noisy": "noisy", "@na": "na", "@todo": "todo", "@manual": "manual"}
)

// zooMarksRows reads the sources of the three zoos under bench (bench/zoo-go/cells, ...) and returns the sorted rows.
func zooMarksRows(bench string) ([]string, error) {
	zoos := []struct{ lang, dir, ext string }{
		{"go", "zoo-go/cells", ".go"},
		{"dart", "zoo-dart/cells/lib", ".dart"},
		{"rust", "zoo-rust/cells/src", ".rs"},
	}
	var rows []string
	for _, z := range zoos {
		root := filepath.Join(bench, z.dir)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, z.ext) {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			within := "-"
			for sc.Scan() {
				text := sc.Text()
				if zooMarksRoleRe.MatchString(text) {
					within = strings.TrimSpace(text)
					if j := strings.Index(within, "//"); j >= 0 {
						within = strings.TrimSpace(within[:j])
					}
					within += " " + zooMarksRoleRe.FindString(text)
				}
				i := strings.Index(text, "// ")
				if i < 0 || !strings.Contains(text[i:], "@") {
					continue
				}
				tokens := zooMarksTokenRe.FindAllString(text[i:], -1)
				kinds := map[string]bool{}
				for _, tok := range tokens {
					name := tok
					if j := strings.Index(tok, "("); j >= 0 {
						name = tok[:j]
					}
					if k, ok := zooMarksKinds[name]; ok {
						kinds[k] = true
					}
					if tok == "@sink(none)" {
						kinds["sink-none"] = true
					}
				}
				for k := range kinds {
					rows = append(rows, strings.Join([]string{z.lang, filepath.ToSlash(rel), k, strings.Join(tokens, " "), within}, "\t"))
				}
			}
			return sc.Err()
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(rows)
	return rows, nil
}

func TestZooMarksLock(t *testing.T) {
	root := filepath.Join(zooRoot(t), "..", "..")
	rows, err := zooMarksRows(filepath.Join(root, "bench"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no marks found: the zoo directories moved?")
	}
	want := strings.Join(rows, "\n") + "\n"
	path := filepath.Join(root, zooMarksLockPath)
	if os.Getenv("ZOO_MARKS_UPDATE") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (ZOO_MARKS_UPDATE=1 go test -run ZooMarksLock ./cmd/rivus writes it)", err)
	}
	if string(got) != want {
		t.Errorf("%s differs from the sources of the zoos:\n%s\n(a mark that loosens the truth needs a %q line in the commit message; ZOO_MARKS_UPDATE=1 writes the file)",
			zooMarksLockPath, zooMarksDiff(strings.Split(strings.TrimSuffix(string(got), "\n"), "\n"), rows), "Zoo-Loosened:")
	}
}

// zooMarksDiff lists the rows only in one side (+ only in the sources, - only in the file), at most 20.
func zooMarksDiff(file, src []string) string {
	count := map[string]int{}
	for _, r := range file {
		count[r]--
	}
	for _, r := range src {
		count[r]++
	}
	var out []string
	for r, n := range count {
		for ; n > 0; n-- {
			out = append(out, "+ "+r)
		}
		for ; n < 0; n++ {
			out = append(out, "- "+r)
		}
	}
	sort.Strings(out)
	if len(out) > 20 {
		out = append(out[:20], fmt.Sprintf("... and %d more", len(out)-20))
	}
	return strings.Join(out, "\n")
}

// A small tree: a mark that is added is seen; the same set of rows is the same file.
func TestZooMarksRowsTool(t *testing.T) {
	bench := t.TempDir()
	dir := filepath.Join(bench, "zoo-go", "cells", "F", "a")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"zoo-dart/cells/lib", "zoo-rust/cells/src"} {
		if err := os.MkdirAll(filepath.Join(bench, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("x := 1 // @sink(t1)\ny := 2 // @sink(none) @nosink(c)\n")
	a, _ := zooMarksRows(bench)
	if len(a) != 1 || !strings.Contains(a[0], "\tsink-none\t") {
		t.Fatalf("rows %q", a)
	}
	write("// a line above\nx := 1 // @sink(t1)\ny := 2 // @sink(none) @nosink(c)\n")
	b, _ := zooMarksRows(bench)
	if strings.Join(a, "|") != strings.Join(b, "|") {
		t.Errorf("an edit above changed the rows: %q vs %q", a, b)
	}
	write("x := 1 // @sink(t1) @miss @ref(X)\ny := 2 // @sink(none) @nosink(c)\n")
	c, _ := zooMarksRows(bench)
	if len(c) != 2 {
		t.Errorf("a @miss was not seen: %q", c)
	}
}

// The check of the commits (scripts/zoo-loosened-check.sh) in a temporary repository: what the commits change in
// the lock as a whole, against the merge base. The rows of a new cell of @sink(none), @miss and @fp need no approval
// (a @miss and a @fp must name an issue), a row added to a cell that the lock already has does, and so does a row of a
// new cell of another kind; the rows of main that a merge brings in are not "added".
func TestZooLoosenedCheck(t *testing.T) {
	root := filepath.Join(zooRoot(t), "..", "..")
	script, _ := filepath.Abs(filepath.Join(root, "scripts/zoo-loosened-check.sh"))
	repo := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	lock := filepath.Join(repo, "bench/zoo/marks.lock")
	write := func(s string) {
		if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lock, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(base string) error {
		cmd := exec.Command("bash", script, base, "HEAD")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		t.Logf("%s", out)
		return err
	}
	const approved = "\n\nZoo-Loosened: test。ユーザーの承認: 2026-01-01"
	run("init", "-q", "-b", "main")
	write("go\ta.go\tmiss\t@miss @ref(#1)\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	first := run("rev-parse", "HEAD")
	base := first
	reset := func(to string) { run("reset", "-q", "--hard", to) }
	// a new cell of @sink(none): nothing is needed
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tsink-none\t@sink(none)\n")
	run("commit", "-qam", "a new cell of @sink(none)")
	if err := check(base); err != nil {
		t.Errorf("the row of a new cell of @sink(none) failed: %v", err)
	}
	reset(base)
	// a new cell of @fp that names an issue: nothing is needed. Without @ref it fails, with or without the line.
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tfp\t@fp @ref(#2)\n")
	run("commit", "-qam", "a new cell of @fp")
	if err := check(base); err != nil {
		t.Errorf("the row of a new cell of @fp with @ref failed: %v", err)
	}
	reset(base)
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tfp\t@fp\n")
	run("commit", "-qam", "a new cell of @fp without the issue"+approved)
	if check(base) == nil {
		t.Error("a new @fp row without @ref passed")
	}
	reset(base)
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tmiss\t@miss\n")
	run("commit", "-qam", "a new cell of @miss without the issue")
	if check(base) == nil {
		t.Error("a new @miss row without @ref passed")
	}
	reset(base)
	// a new cell of another kind (@noisy): fails without the line, passes with it
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tnoisy\t@noisy\n")
	run("commit", "-qam", "a new cell of @noisy")
	if check(base) == nil {
		t.Error("the row of a new cell of @noisy passed without Zoo-Loosened")
	}
	run("commit", "-q", "--amend", "-m", "a new cell of @noisy"+approved)
	if err := check(base); err != nil {
		t.Errorf("the row of a new cell of @noisy with Zoo-Loosened failed: %v", err)
	}
	reset(base)
	// a row added to a cell that the lock already has (a mark added, or a mark changed): fails without the line
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\ta.go\tsink-none\t@sink(none)\n")
	run("commit", "-qam", "a mark added to an existing cell")
	if check(base) == nil {
		t.Error("a row added to an existing cell passed without Zoo-Loosened")
	}
	run("commit", "-q", "--amend", "-m", "a mark added to an existing cell"+approved)
	if err := check(base); err != nil {
		t.Errorf("a row added to an existing cell with Zoo-Loosened failed: %v", err)
	}
	reset(base)
	write("go\ta.go\tmiss\t@miss(depth_limit) @ref(#1)\n")
	run("commit", "-qam", "a mark of an existing cell changed")
	if check(base) == nil {
		t.Error("a mark of an existing cell that was changed passed without Zoo-Loosened")
	}
	reset(base)
	// a wrong form of the line (no approval date) fails
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\ta.go\tsink-none\t@sink(none)\n")
	run("commit", "-qam", "add\n\nZoo-Loosened: no date")
	if check(base) == nil {
		t.Error("a Zoo-Loosened line without the approval date passed")
	}
	reset(base)
	// the rows that main brings in with a merge are not "added" (main checked them): passes with nothing
	run("checkout", "-q", "-b", "feat")
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tsink-none\t@sink(none)\n")
	run("commit", "-qam", "a new cell on the branch")
	run("checkout", "-q", "main")
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tc.go\tnoisy\t@noisy\n")
	run("commit", "-qam", "main adds a @noisy row"+approved)
	mainTip := run("rev-parse", "HEAD")
	run("checkout", "-q", "feat")
	// both branches edited the end of the file: the merge stops, and the rows of the two are put together by hand
	merge := exec.Command("git", "merge", "-q", "--no-ff", "--no-commit", "main")
	merge.Dir = repo
	merge.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	_, _ = merge.CombinedOutput()
	if _, err := os.Stat(filepath.Join(repo, ".git", "MERGE_HEAD")); err != nil {
		t.Fatalf("the merge of main did not start (the commit would not be a merge): %v", err)
	}
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tsink-none\t@sink(none)\ngo\tc.go\tnoisy\t@noisy\n")
	run("add", ".")
	run("commit", "-qm", "merge main")
	if err := check(mainTip); err != nil {
		t.Errorf("a merge of main failed: %v", err)
	}
	run("checkout", "-q", "main")
	reset(first)
	// CI-1: delete the file in one commit and make it again with a new row in the next one: fails
	write("go\ta.go\tmiss\t@miss @ref(#1)\n")
	run("rm", "-q", "bench/zoo/marks.lock")
	run("commit", "-qm", "delete the lock")
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tsink-none\t@sink(none)\n")
	run("add", ".")
	run("commit", "-qm", "make it again")
	if check(base) == nil {
		t.Error("delete and make again passed")
	}
	reset(first)
	// a base that is not usable fails
	if check("0000000000000000000000000000000000000000") == nil {
		t.Error("an unusable base passed")
	}
	// a row is removed: passes with nothing
	write("")
	run("commit", "-qam", "fix a mark")
	if err := check(base); err != nil {
		t.Errorf("a removed row failed: %v", err)
	}
	// ZOO-1: a column is added. Only the columns that both files have are compared.
	reset(first)
	write("go\ta.go\tmiss\t@miss @ref(#1)\tfunc vuln()\n")
	run("commit", "-qam", "add a column")
	if err := check(base); err != nil {
		t.Errorf("a column added with the same rows failed: %v", err)
	}
	// a column and a row of a new @noisy cell at once: the row needs the line
	reset(first)
	write("go\ta.go\tmiss\t@miss @ref(#1)\tfunc vuln()\ngo\tb.go\tnoisy\t@noisy\tfunc f()\n")
	run("commit", "-qam", "add a column and a row")
	if check(base) == nil {
		t.Error("a column and a @noisy row added at once passed")
	}
	// 5 columns: a @sink(none) moved to another case (the 5th column differs): the cell is old, so it fails
	reset(first)
	write("go\ta.go\tsink-none\t@sink(none)\tfunc vuln()\n")
	run("commit", "-qam", "five columns")
	five := run("rev-parse", "HEAD")
	write("go\ta.go\tsink-none\t@sink(none)\tfunc fixed()\n")
	run("commit", "-qam", "move the mark")
	if check(five) == nil {
		t.Error("a mark moved to another case passed")
	}
	// 5 -> 4 columns: fails
	reset(five)
	write("go\ta.go\tsink-none\t@sink(none)\n")
	run("commit", "-qam", "four columns")
	if check(five) == nil {
		t.Error("a column taken away passed")
	}
}
