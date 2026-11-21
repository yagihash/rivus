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
//	<lang> TAB <file> TAB <kind> TAB <the annotations of the line>
//
// (no line number, so that an edit above does not change the rows). TestZooMarksLock holds the file to the sources;
// .github/scripts/zoo-loosened-check.sh fails a commit that adds a row without a "Zoo-Loosened:" line in its message.
// A row that goes away (a mark that is fixed) needs nothing. To write the file again: ZOO_MARKS_UPDATE=1.

const zooMarksLockPath = "bench/zoo/marks.lock"

var (
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
			for sc.Scan() {
				text := sc.Text()
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
					rows = append(rows, strings.Join([]string{z.lang, filepath.ToSlash(rel), k, strings.Join(tokens, " ")}, "\t"))
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

// The check of the commits (.github/scripts/zoo-loosened-check.sh) in a temporary repository.
func TestZooLoosenedCheck(t *testing.T) {
	root := filepath.Join(zooRoot(t), "..", "..")
	script, _ := filepath.Abs(filepath.Join(root, ".github/scripts/zoo-loosened-check.sh"))
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
	run("init", "-q")
	write("go\ta.go\tmiss\t@miss\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")
	// (b) a row is added without the line: fails
	write("go\ta.go\tmiss\t@miss\ngo\tb.go\tfp\t@fp\n")
	run("commit", "-qam", "add a mark")
	if check(base) == nil {
		t.Error("a row added without Zoo-Loosened passed")
	}
	// with the line: passes
	run("commit", "-q", "--amend", "-m", "add a mark\n\nZoo-Loosened: test。ユーザーの承認: 2026-01-01")
	if err := check(base); err != nil {
		t.Errorf("a row added with Zoo-Loosened failed: %v", err)
	}
	// (c) a row is removed: passes with nothing
	base = run("rev-parse", "HEAD")
	write("go\ta.go\tmiss\t@miss\n")
	run("commit", "-qam", "fix a mark")
	if err := check(base); err != nil {
		t.Errorf("a removed row failed: %v", err)
	}
}
