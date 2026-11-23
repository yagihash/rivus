package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/yagihash/rivus/internal/docsgen"
)

// docs/issues. A file is `<number>-<name>.md` (the number is unique; the header has the five keys, and `make docs-gen`
// makes a row of the table from it) or `model-<package>-<name>.md` (no number; a pull request that adds a model names
// it from a zoo mark by @ref(model-...) and does not touch the tables; it has the sections of docs/issues/README.md).
// The next number is the largest in docs/issues plus one: no line of ISSUES.md holds it, so that pull requests do not
// conflict on it.
func TestIssuesFiles(t *testing.T) {
	dir := filepath.Join("..", "..", "docs", "issues")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	numbered := map[int]string{}
	nameRe := regexp.MustCompile(`^model-[a-z0-9][a-z0-9._-]*\.md$`)
	numRe := regexp.MustCompile(`^([0-9]+)-[^/]+\.md$`)
	for _, e := range ents {
		n := e.Name()
		if n == "README.md" {
			continue
		}
		if m := numRe.FindStringSubmatch(n); m != nil {
			num, _ := strconv.Atoi(m[1])
			if other, dup := numbered[num]; dup {
				t.Errorf("the number %d is the number of %s and of %s", num, other, n)
			}
			numbered[num] = n
			continue
		}
		if !nameRe.MatchString(n) {
			t.Errorf("docs/issues/%s is neither <number>-<name>.md nor model-<package>-<name>.md", n)
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		if !regexp.MustCompile(`(?m)^# .+`).MatchString(body) {
			t.Errorf("%s has no title line (# ...)", n)
		}
		for _, sec := range []string{"## 何が見逃されるか", "## 原因", "## 再現するセル"} {
			if !regexp.MustCompile("(?m)^" + regexp.QuoteMeta(sec)).MatchString(body) {
				t.Errorf("%s has no %q section (docs/issues/README.md)", n, sec)
			}
		}
	}
	issues, errs := docsgen.Load(dir)
	for _, e := range errs {
		t.Error(e)
	}
	if len(issues) != len(numbered) {
		t.Errorf("docsgen read %d numbered files, the directory has %d", len(issues), len(numbered))
	}
	valid := map[string]bool{}
	for _, st := range docsgen.States {
		valid[st] = true
	}
	for _, is := range issues {
		for _, k := range docsgen.Keys[:5] { // 題 is optional
			if _, ok := is.Fields[k]; !ok {
				t.Errorf("%s: the header has no %q line (docs/issues/README.md)", is.File, k)
			}
		}
		if !valid[is.State] {
			t.Errorf("%s: 状態 is %q; the first word must be one of %v", is.File, is.Fields["状態"], docsgen.States)
		}
		if is.Title == "" {
			t.Errorf("%s: no title", is.File)
		}
	}
	// the generated table has one row for every numbered file, and no other
	rowRe := regexp.MustCompile(`(?m)^\| \[#([0-9]+)\]\(issues/([^)]+)\) \|`)
	rows := map[int]string{}
	for _, m := range rowRe.FindAllStringSubmatch(docsgen.Table(issues), -1) {
		num, _ := strconv.Atoi(m[1])
		if _, dup := rows[num]; dup {
			t.Errorf("the table lists #%d twice", num)
		}
		rows[num] = m[2]
	}
	for num, n := range numbered {
		if rows[num] != n {
			t.Errorf("docs/issues/%s: the generated table has the row %q", n, rows[num])
		}
	}
	if len(rows) != len(numbered) {
		t.Errorf("the table has %d rows, the directory has %d numbered files", len(rows), len(numbered))
	}
	// ISSUES.md is a guide: it holds no table and no next number (a line that every pull request edits conflicts)
	tab, err := os.ReadFile(filepath.Join("..", "..", "docs", "ISSUES.md"))
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?m)^\| \[#[0-9]+\]`).Match(tab) {
		t.Error("ISSUES.md has a row of the table: the table is made by `make docs-gen` from docs/issues (add a file, not a row)")
	}
	if regexp.MustCompile(`新しい課題は [0-9]+ から`).Match(tab) {
		t.Error("ISSUES.md holds the next number: it is the largest number in docs/issues plus one (a line with a number conflicts between pull requests)")
	}
}
