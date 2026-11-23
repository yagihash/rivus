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
// and @sink(none) of the three zoos are listed in bench/zoo/marks.d, one file for a cell file
// (bench/zoo/marks.d/<lang>/<path of the cell file>.lock: so that two pull requests that add cells do not touch the same file),
// one row for a mark of a line:
//
//	<lang> TAB <file> TAB <kind> TAB <the annotations of the line> TAB <the header of the case the line is in>
//
// (no line number, so that an edit above does not change the rows; the last column is the line of the @vuln /
// @fixed / @decoy that comes before it, or "-", so that a mark that is moved to another case is a new row). TestZooMarksLock holds the file to the sources;
// scripts/zoo-loosened-check.sh fails a PR whose rows (against the merge base) loosen the zoo without a "Zoo-Loosened:"
// line in a commit message (a row added to a cell that the lock has, a row of a new cell of a kind other than @sink(none),
// @miss and @fp), and a new @miss or @fp row that names no issue file. A row that goes away (a mark that is fixed) needs nothing. To write the files again: ZOO_MARKS_UPDATE=1.

const zooMarksDirPath = "bench/zoo/marks.d"

var (
	zooMarksRoleRe  = regexp.MustCompile(`@(vuln|fixed|decoy)\b`)
	zooMarksTokenRe = regexp.MustCompile(`@[a-z]+(?:\([^)]*\))?`)
	zooMarksKinds   = map[string]string{"@miss": "miss", "@fp": "fp", "@noisy": "noisy", "@na": "na", "@todo": "todo", "@manual": "manual"}
)

// zooMarksRows reads the sources of the three zoos under bench (bench/zoo-go/cells, ...) and returns the sorted rows.
func zooMarksRows(bench string) ([]string, error) {
	zoos := []struct{ lang, dir, ext, prefix string }{
		{"go", "zoo-go/cells", ".go", ""},
		{"dart", "zoo-dart/cells/lib", ".dart", ""},
		{"rust", "zoo-rust/cells/src", ".rs", ""},
	}
	// the packages of the zoo (bench/zoo-go-pkg/<package>, bench/zoo-rust-pkg/<package>): found by the directories; the file of a
	// cell is pkg/<package>/<file>, so that its lock is bench/zoo/marks.d/<lang>/pkg/<package>/<file>.lock
	for _, p := range []struct{ lang, dir, sub, ext string }{{"go", "zoo-go-pkg", "cells", ".go"}, {"rust", "zoo-rust-pkg", "cells/src", ".rs"}} {
		names, _ := os.ReadDir(filepath.Join(bench, p.dir))
		for _, n := range names {
			if n.IsDir() {
				zoos = append(zoos, struct{ lang, dir, ext, prefix string }{p.lang, filepath.Join(p.dir, n.Name(), p.sub), p.ext, "pkg/" + n.Name() + "/"})
			}
		}
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
			rel = z.prefix + rel
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
				// a mark written outside the cell files: the lines of the header of a Rust file (`//! @todo(...)`, e.g. the top of
				// lib.rs of the Rust zoo, which is not a cell). One row for a line, the annotations are the whole line, so that a
				// change of the text is a new row. A `@todo(...: PARITY: ...)` there is the kind "todo-parity" (a cell that another
				// language has and this one has not: no approval, docs/ROADMAP.md); the rest (@na, @noisy, @manual, @miss, @fp, a @todo that is
				// not PARITY) is the kind of the mark.
				if z.lang == "rust" {
					if h := strings.TrimSpace(text); strings.HasPrefix(h, "//!") {
						body := strings.TrimSpace(strings.TrimPrefix(h, "//!"))
						if strings.HasPrefix(body, "@") {
							name := zooMarksTokenRe.FindString(body)
							if j := strings.Index(name, "("); j >= 0 {
								name = name[:j]
							}
							k, ok := zooMarksKinds[name]
							if ok && k == "todo" && strings.Contains(body, "PARITY") {
								k = "todo-parity"
							}
							if ok {
								rows = append(rows, strings.Join([]string{z.lang, filepath.ToSlash(rel), k, body, "-"}, "\t"))
							}
						}
						continue
					}
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

// zooMarksFiles groups the rows by the file that holds them: "<lang>/<cell file>.lock" -> the text of the file.
func zooMarksFiles(rows []string) map[string]string {
	out := map[string]string{}
	for _, r := range rows {
		f := strings.SplitN(r, "\t", 3)
		name := f[0] + "/" + f[1] + ".lock"
		out[name] += r + "\n"
	}
	return out
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
	want := zooMarksFiles(rows)
	dir := filepath.Join(root, zooMarksDirPath)
	if os.Getenv("ZOO_MARKS_UPDATE") == "1" {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		for name, text := range want {
			p := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	got := map[string]string{}
	err = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		b, err := os.ReadFile(p)
		got[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatalf("%v (ZOO_MARKS_UPDATE=1 go test -run ZooMarksLock ./cmd/rivus writes the files)", err)
	}
	var fileRows []string
	for name, text := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s/%s is the lock of a cell that has no mark in the sources (or a stray file): ZOO_MARKS_UPDATE=1 writes the files again", zooMarksDirPath, name)
		}
		fileRows = append(fileRows, strings.Split(strings.TrimSuffix(text, "\n"), "\n")...)
	}
	if diff := zooMarksDiff(fileRows, rows); diff != "" {
		t.Errorf("%s differs from the sources of the zoos:\n%s\n(a mark that loosens the truth needs a %q line in the commit message; ZOO_MARKS_UPDATE=1 writes the files)",
			zooMarksDirPath, diff, "Zoo-Loosened:")
	}
	for name, text := range want {
		if got[name] != text {
			t.Errorf("%s/%s differs from the sources of the zoos (the rows are in the file of their cell, in the order of the sort): ZOO_MARKS_UPDATE=1 writes the files", zooMarksDirPath, name)
		}
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
	// write puts the rows (the text of the old single file) into the lock files of their cells, and stages everything
	write := func(s string) {
		dir := filepath.Join(repo, "bench/zoo/marks.d")
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		files := map[string]string{}
		for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
			if line == "" {
				continue
			}
			f := strings.SplitN(line, "\t", 3)
			files[f[0]+"/"+f[1]+".lock"] += line + "\n"
		}
		for name, text := range files {
			p := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		run("add", "-A")
	}
	// issue puts a file in docs/issues, and stages it
	issue := func(name, body string) {
		p := filepath.Join(repo, "docs/issues", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "-A")
	}
	rawfile := func(name, body string) {
		p := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "-A")
	}
	const template = "# the title\n\n## 何が見逃されるか\n\nx\n\n## 原因\n\nx\n\n## 再現するセル\n\nx\n"
	check := func(base string) error {
		cmd := exec.Command("bash", script, base, "HEAD")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		t.Logf("%s", out)
		return err
	}
	const approved = "\n\nZoo-Loosened: test。ユーザーの承認: 2026-01-01"
	run("init", "-q", "-b", "main")
	issue("1-a.md", "# a\n")
	issue("2-b.md", "# b\n")
	issue("model-p-x.md", template)
	issue("model-p-nosection.md", "# the title\n\n## 何が見逃されるか\n\nx\n")
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\n")
	run("commit", "-q", "-m", "base")
	first := run("rev-parse", "HEAD")
	base := first
	reset := func(to string) { run("reset", "-q", "--hard", to) }
	// a new cell of @sink(none): nothing is needed
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tsink-none\t@sink(none)\n")
	run("commit", "-qam", "a new cell of @sink(none)")
	if err := check(base); err != nil {
		t.Errorf("the row of a new cell of @sink(none) failed: %v", err)
	}
	reset(base)
	// a new cell of @fp that names an issue: nothing is needed. Without @ref it fails, with or without the line.
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tfp\t@fp @ref(#2)\n")
	run("commit", "-qam", "a new cell of @fp")
	if err := check(base); err != nil {
		t.Errorf("the row of a new cell of @fp with @ref failed: %v", err)
	}
	reset(base)
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tfp\t@fp\n")
	run("commit", "-qam", "a new cell of @fp without the issue"+approved)
	if check(base) == nil {
		t.Error("a new @fp row without @ref passed")
	}
	reset(base)
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tmiss\t@miss\n")
	run("commit", "-qam", "a new cell of @miss without the issue")
	if check(base) == nil {
		t.Error("a new @miss row without @ref passed")
	}
	reset(base)
	// the issue of a new row: a name (model-<package>-<name>) with its file and the sections passes; no file, a file without
	// the sections, and a number with no file fail (a number is stricter than before: the file must be there)
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\ngo\tb.go\tmiss\t@miss @ref(model-p-x)\n")
	run("commit", "-qam", "a new @miss with the name of an issue")
	if err := check(base); err != nil {
		t.Errorf("a new @miss with @ref(model-p-x) and its file failed: %v", err)
	}
	reset(base)
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\ngo\tb.go\tfp\t@fp @ref(model-p-nofile)\n")
	run("commit", "-qam", "a name with no file")
	if check(base) == nil {
		t.Error("a new @fp with the name of an issue that has no file passed")
	}
	reset(base)
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\ngo\tb.go\tmiss\t@miss @ref(model-p-nosection)\n")
	run("commit", "-qam", "a name whose file lacks the sections")
	if check(base) == nil {
		t.Error("a new @miss with a file that does not have the sections of the template passed")
	}
	reset(base)
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\ngo\tb.go\tmiss\t@miss @ref(#999)\n")
	run("commit", "-qam", "a number with no file")
	if check(base) == nil {
		t.Error("a new @miss with a number that has no file passed")
	}
	reset(base)
	// a list of references: the issue and another name (PARITY) pass; the other name alone fails
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\ngo\tb.go\tmiss\t@miss @ref(#2,PARITY)\n")
	run("commit", "-qam", "a list of references")
	if err := check(base); err != nil {
		t.Errorf("@ref(#2,PARITY) failed: %v", err)
	}
	reset(base)
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\ngo\tb.go\tmiss\t@miss @ref(PARITY)\n")
	run("commit", "-qam", "a name that is not an issue")
	if check(base) == nil {
		t.Error("@ref(PARITY) alone passed")
	}
	reset(base)
	// the rows that are there already (an issue that has no file now, an old name) are not checked again
	run("rm", "-q", "docs/issues/1-a.md")
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\n")
	run("commit", "-qm", "the file of the issue of an old row goes away")
	if err := check(base); err != nil {
		t.Errorf("an old row was checked again: %v", err)
	}
	reset(base)
	// a new cell of another kind (@noisy): fails without the line, passes with it
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tnoisy\t@noisy\n")
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
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\ta.go\tsink-none\t@sink(none)\n")
	run("commit", "-qam", "a mark added to an existing cell")
	if check(base) == nil {
		t.Error("a row added to an existing cell passed without Zoo-Loosened")
	}
	run("commit", "-q", "--amend", "-m", "a mark added to an existing cell"+approved)
	if err := check(base); err != nil {
		t.Errorf("a row added to an existing cell with Zoo-Loosened failed: %v", err)
	}
	reset(base)
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss(depth_limit) @ref(#1)\n")
	run("commit", "-qam", "a mark of an existing cell changed")
	if check(base) == nil {
		t.Error("a mark of an existing cell that was changed passed without Zoo-Loosened")
	}
	reset(base)
	// a wrong form of the line (no approval date) fails
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\ta.go\tsink-none\t@sink(none)\n")
	run("commit", "-qam", "add\n\nZoo-Loosened: no date")
	if check(base) == nil {
		t.Error("a Zoo-Loosened line without the approval date passed")
	}
	reset(base)
	// the rows that main brings in with a merge are not "added" (main checked them): passes with nothing
	run("checkout", "-q", "-b", "feat")
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tsink-none\t@sink(none)\n")
	run("commit", "-qam", "a new cell on the branch")
	run("checkout", "-q", "main")
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\tc.go\tnoisy\t@noisy\n")
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
	write("go\tz.go\tsink-none\t@sink(none)\ngo\ta.go\tmiss\t@miss @ref(#1)\ngo\tb.go\tsink-none\t@sink(none)\ngo\tc.go\tnoisy\t@noisy\n")
	run("add", ".")
	run("commit", "-qm", "merge main")
	if err := check(mainTip); err != nil {
		t.Errorf("a merge of main failed: %v", err)
	}
	run("checkout", "-q", "main")
	reset(first)
	// CI-1: delete the file in one commit and make it again with a new row in the next one: fails
	run("rm", "-q", "bench/zoo/marks.d/go/z.go.lock")
	run("commit", "-qm", "delete the lock of a cell")
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\tfunc f()\ngo\tz.go\tnoisy\t@noisy\n")
	run("commit", "-qm", "make it again")
	if check(base) == nil {
		t.Error("delete and make again passed")
	}
	// the lock file of a cell that goes away (and stays away) needs nothing
	reset(first)
	run("rm", "-q", "bench/zoo/marks.d/go/z.go.lock")
	run("commit", "-qm", "the marks of a cell are fixed")
	if err := check(base); err != nil {
		t.Errorf("the lock file of a cell that went away failed: %v", err)
	}
	// moving the lock of a cell to another cell (the rows say the old cell): fails
	reset(first)
	run("mv", "bench/zoo/marks.d/go/z.go.lock", "bench/zoo/marks.d/go/y.go.lock")
	run("commit", "-qm", "move the lock file")
	if check(base) == nil {
		t.Error("a lock file with the rows of another cell passed")
	}
	// the cell file moved (the rows and the lock follow it): a new cell, as before the split: @noisy needs the line
	reset(first)
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\ty.go\tsink-none\t@sink(none)\n")
	run("commit", "-qm", "a cell moved")
	if err := check(base); err != nil {
		t.Errorf("a moved cell with @sink(none) is a new cell and needs nothing: %v", err)
	}
	// a stray file in marks.d and the single file of the old layout at the head: fail
	reset(first)
	rawfile("bench/zoo/marks.d/go/a.go.txt", "x\n")
	run("commit", "-qm", "a stray file")
	if check(base) == nil {
		t.Error("a stray file in marks.d passed")
	}
	reset(first)
	rawfile("bench/zoo/marks.lock", "go\ta.go\tmiss\t@miss @ref(#1)\n")
	run("commit", "-qm", "the old file again")
	if check(base) == nil {
		t.Error("the single lock file at the head passed")
	}
	reset(first)
	// a base that is not usable fails
	if check("0000000000000000000000000000000000000000") == nil {
		t.Error("an unusable base passed")
	}
	// a row is removed: passes with nothing
	write("go\ta.go\tmiss\t@miss @ref(#1)\n")
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
	// the split itself: the base has the single file of the old layout, the head the same rows in the lock files of the cells
	reset(first)
	run("rm", "-rq", "bench/zoo/marks.d")
	rawfile("bench/zoo/marks.lock", "go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\n")
	run("commit", "-qm", "the old layout")
	oldLayout := run("rev-parse", "HEAD")
	run("rm", "-q", "bench/zoo/marks.lock")
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\n")
	run("commit", "-qm", "split")
	if err := check(oldLayout); err != nil {
		t.Errorf("the split of the single file with the same rows failed: %v", err)
	}
	write("go\ta.go\tmiss\t@miss @ref(#1)\ngo\tz.go\tsink-none\t@sink(none)\ngo\tz.go\tnoisy\t@noisy\n")
	run("commit", "-qm", "split and a row of an existing cell added")
	if check(oldLayout) == nil {
		t.Error("a row added to an existing cell in the commit that splits the file passed")
	}
}

// The marks that a Rust file writes in its header (`//! @todo(...)` of lib.rs) are rows of the lock too: a `@todo(...: PARITY: ...)`
// there needs no line (a cell that another language has and this one has not yet: the user, 2026-11-23); the rest needs it.
func TestZooMarksRowsHeader(t *testing.T) {
	bench := t.TempDir()
	for _, d := range []string{"zoo-go/cells", "zoo-dart/cells/lib", "zoo-rust/cells/src"} {
		if err := os.MkdirAll(filepath.Join(bench, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	src := "//! prose that names `@na` and `@todo` in the middle of a line is not a mark\n//! @todo(F/a/x: PARITY: one (two))\n//! @todo(F/a/y: later)\n//! @na(F/a/z: no)\n//! @noisy(F/a/w: no)\n"
	if err := os.WriteFile(filepath.Join(bench, "zoo-rust/cells/src/lib.rs"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := zooMarksRows(bench)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(rows, "\n")
	for _, w := range []string{"rust\tlib.rs\ttodo-parity\t@todo(F/a/x: PARITY: one (two))\t-", "rust\tlib.rs\ttodo\t@todo(F/a/y: later)\t-", "rust\tlib.rs\tna\t", "rust\tlib.rs\tnoisy\t"} {
		if !strings.Contains(got, w) {
			t.Errorf("no row %q in\n%s", w, got)
		}
	}
	if len(rows) != 4 {
		t.Errorf("%d rows, want 4:\n%s", len(rows), got)
	}
}

// The check of the commits for the rows of lib.rs: PARITY passes, the rest fails without the line and passes with it, and a row that goes away passes.
func TestZooLoosenedCheckHeader(t *testing.T) {
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
	write := func(rows ...string) {
		p := filepath.Join(repo, "bench/zoo/marks.d/rust/lib.rs.lock")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "-A")
	}
	check := func(base string) error {
		cmd := exec.Command("bash", script, base, "HEAD")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		t.Logf("%s", out)
		return err
	}
	const approved = "\n\nZoo-Loosened: test。ユーザーの承認: 2026-01-01"
	parity := func(n string) string { return "rust\tlib.rs\ttodo-parity\t@todo(F/a/" + n + ": PARITY: x)\t-" }
	other := "rust\tlib.rs\ttodo\t@todo(F/a/o: later)\t-"
	na := "rust\tlib.rs\tna\t@na(F/a/n: no)\t-"
	run("init", "-q", "-b", "main")
	write(parity("p1"), na)
	run("commit", "-qm", "base")
	base := run("rev-parse", "HEAD")
	reset := func() { run("reset", "-q", "--hard", base) }
	write(parity("p1"), na, parity("p2"))
	run("commit", "-qam", "a PARITY todo is added to lib.rs")
	if err := check(base); err != nil {
		t.Errorf("a PARITY @todo added to lib.rs failed: %v", err)
	}
	reset()
	write(parity("p1"), na, other)
	run("commit", "-qam", "a todo that is not PARITY")
	if check(base) == nil {
		t.Error("a @todo that is not PARITY, added to lib.rs, passed without Zoo-Loosened")
	}
	run("commit", "-q", "--amend", "-m", "a todo that is not PARITY"+approved)
	if err := check(base); err != nil {
		t.Errorf("a @todo that is not PARITY with Zoo-Loosened failed: %v", err)
	}
	reset()
	write(parity("p1"), na, "rust\tlib.rs\tna\t@na(F/a/n2: no)\t-")
	run("commit", "-qam", "an @na is added")
	if check(base) == nil {
		t.Error("an @na added to lib.rs passed without Zoo-Loosened")
	}
	run("commit", "-q", "--amend", "-m", "an @na is added"+approved)
	if err := check(base); err != nil {
		t.Errorf("an @na with Zoo-Loosened failed: %v", err)
	}
	reset()
	write(parity("p1"), "rust\tlib.rs\tna\t@na(F/a/n: a changed text)\t-")
	run("commit", "-qam", "the text of an @na is changed")
	if check(base) == nil {
		t.Error("a changed @na of lib.rs passed without Zoo-Loosened")
	}
	reset()
	write(na)
	run("commit", "-qam", "a PARITY todo goes away")
	if err := check(base); err != nil {
		t.Errorf("a PARITY @todo that goes away failed: %v", err)
	}
	// the lock is written for the first time for a lib.rs that has the marks already: not added; a new mark in the same commit is
	reset()
	run("rm", "-q", "-f", "bench/zoo/marks.d/rust/lib.rs.lock")
	src := filepath.Join(repo, "bench/zoo-rust/cells/src/lib.rs")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("//! @na(F/a/n: no)\n//! @na(F/a/m: no)\n//! @na(F/a/l: no)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "lib.rs without a lock")
	base2 := run("rev-parse", "HEAD")
	lock := func(rows ...string) {
		write(rows...)
		run("commit", "-qm", "the lock of lib.rs")
	}
	lock("rust\tlib.rs\tna\t@na(F/a/n: no)\t-", "rust\tlib.rs\tna\t@na(F/a/m: no)\t-", "rust\tlib.rs\tna\t@na(F/a/l: no)\t-")
	if err := check(base2); err != nil {
		t.Errorf("the first lock of a lib.rs that has the @na rows already failed: %v", err)
	}
	run("reset", "-q", "--hard", base2)
	if err := os.WriteFile(src, []byte("//! @na(F/a/n: no)\n//! @na(F/a/m: no)\n//! @na(F/a/l: no)\n//! @na(F/a/new: no)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock("rust\tlib.rs\tna\t@na(F/a/n: no)\t-", "rust\tlib.rs\tna\t@na(F/a/m: no)\t-", "rust\tlib.rs\tna\t@na(F/a/l: no)\t-", "rust\tlib.rs\tna\t@na(F/a/new: no)\t-")
	if check(base2) == nil {
		t.Error("a new @na in the commit that makes the first lock passed without Zoo-Loosened")
	}
}
