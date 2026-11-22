package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// docs/issues and the tables of docs/ISSUES.md. A file is `<number>-<name>.md` (the number is unique, and the table has a
// row for it with a link to that file) or `model-<package>-<name>.md` (no number; a pull request that adds a model names
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
	tab, err := os.ReadFile(filepath.Join("..", "..", "docs", "ISSUES.md"))
	if err != nil {
		t.Fatal(err)
	}
	rowRe := regexp.MustCompile(`^\| \[#([0-9]+)\]\(issues/([^)]+)\)`)
	listed := map[int]bool{}
	for _, l := range strings.Split(string(tab), "\n") {
		m := rowRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		num, _ := strconv.Atoi(m[1])
		if listed[num] {
			t.Errorf("ISSUES.md lists #%d twice", num)
		}
		listed[num] = true
		if numbered[num] != m[2] {
			t.Errorf("ISSUES.md row #%d links to %s, and the file of the number is %q", num, m[2], numbered[num])
		}
	}
	for num, n := range numbered {
		if !listed[num] {
			t.Errorf("docs/issues/%s has no row in the tables of ISSUES.md", n)
		}
	}
	if regexp.MustCompile(`新しい課題は [0-9]+ から`).Match(tab) {
		t.Error("ISSUES.md holds the next number: it is the largest number in docs/issues plus one (a line with a number conflicts between pull requests)")
	}
}
