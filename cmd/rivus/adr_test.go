package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/docsgen"
)

// docs/adr. A file is `<number>-<name>.md`: the number is unique and equals the number of the title line
// `# ADR-NNN: <title>`; the header has 状態 and 関連 (docs/adr/README.md); `make docs-gen WHAT=adr` makes a row of the table
// from it. The next number is the largest on main plus one: no line of DECISIONS.md or ADR-INDEX.md holds it, so that
// pull requests do not conflict on it (a collision of numbers fails here, and the later pull request renumbers).
func TestAdrFiles(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	dir := filepath.Join(root, "docs", "adr")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	numbered := map[int]string{}
	for _, e := range ents {
		n := e.Name()
		if n == "README.md" {
			continue
		}
		num, ok := docsgen.AdrName(n)
		if !ok {
			t.Errorf("docs/adr/%s is not <3-digit number>-<name>.md (lowercase letters, digits and -)", n)
			continue
		}
		if other, dup := numbered[num]; dup {
			t.Errorf("the number %d is the number of %s and of %s", num, other, n)
		}
		numbered[num] = n
	}
	adrs, errs := docsgen.LoadAdr(dir)
	for _, e := range errs {
		t.Error(e)
	}
	if len(adrs) != len(numbered) {
		t.Errorf("docsgen read %d files, the directory has %d", len(adrs), len(numbered))
	}
	valid := map[string]bool{}
	for _, st := range docsgen.AdrStates {
		valid[st] = true
	}
	for _, a := range adrs {
		if a.HeadNum != a.Number {
			t.Errorf("%s: the title line has ADR-%03d", a.File, a.HeadNum)
		}
		for _, k := range docsgen.AdrKeys[:2] {
			if _, ok := a.Fields[k]; !ok {
				t.Errorf("%s: the header has no %q line (docs/adr/README.md)", a.File, k)
			}
		}
		if !valid[a.State] {
			t.Errorf("%s: 状態 is %q; it must start with one of %v", a.File, a.Fields["状態"], docsgen.AdrStates)
		}
		if (a.State == "一部覆った" || a.State == "覆った") && len(a.Overrides) == 0 && a.Fields["状態の詳細"] == "" {
			t.Errorf("%s: 状態 %q names no ADR (→ADR-MMM) and the file has no 状態の詳細", a.File, a.State)
		}
		for _, m := range a.Overrides {
			if _, ok := numbered[m]; !ok {
				t.Errorf("%s: 状態 points to ADR-%03d, which has no file", a.File, m)
			}
			if m == a.Number {
				t.Errorf("%s: 状態 points to itself", a.File)
			}
		}
		if a.Title == "" {
			t.Errorf("%s: no title", a.File)
		}
	}
	// the generated table has one row for every file, and no other
	rowRe := regexp.MustCompile(`(?m)^\| ADR-([0-9]+) \| .* \| \[([^\]]+)\]\(adr/([^)]+)\) \|$`)
	rows := map[int]string{}
	for _, m := range rowRe.FindAllStringSubmatch(docsgen.AdrTable(adrs), -1) {
		num, _ := strconv.Atoi(m[1])
		if _, dup := rows[num]; dup {
			t.Errorf("the table lists ADR-%d twice", num)
		}
		rows[num] = m[3]
	}
	for num, n := range numbered {
		if rows[num] != n {
			t.Errorf("docs/adr/%s: the generated table has the row %q", n, rows[num])
		}
	}
	if len(rows) != len(numbered) {
		t.Errorf("the table has %d rows, the directory has %d files", len(rows), len(numbered))
	}
	// DECISIONS.md and ADR-INDEX.md are guides: no ADR section, no row of the table (a line that every pull request edits conflicts)
	for _, f := range []string{"DECISIONS.md", "ADR-INDEX.md"} {
		b, err := os.ReadFile(filepath.Join(root, "docs", f))
		if err != nil {
			t.Fatal(err)
		}
		if regexp.MustCompile(`(?m)^#{2,3} ADR-[0-9]+`).Match(b) {
			t.Errorf("%s has an ADR section: an ADR is a file of docs/adr (add a file, not a section)", f)
		}
		if regexp.MustCompile(`(?m)^\| (ADR-)?[0-9]{3} \|`).Match(b) {
			t.Errorf("%s has a row of the table: the table is made by `make docs-gen` from docs/adr", f)
		}
	}
}

// TestAdrReferences: every `ADR-NNN` written in the repository (documents, code, tests, scripts) has a file in docs/adr.
// Ranges and examples (`ADR-NNN`, `ADR-001〜100`) have no number after `ADR-` or are skipped by the allow list.
func TestAdrReferences(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	have := map[int]bool{}
	ents, err := os.ReadDir(filepath.Join(root, "docs", "adr"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if n, ok := docsgen.AdrName(e.Name()); ok {
			have[n] = true
		}
	}
	// references that point to no file today and are not fixed here (path from the root -> numbers); empty: none
	allow := map[string][]int{}
	files, err := adrScanFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for _, msg := range adrMissing(rel, string(b), have, allow) {
			t.Error(msg)
		}
	}
}

var adrRefRe = regexp.MustCompile(`ADR-([0-9]+)`)

// adrMissing: the messages for the `ADR-NNN` of text that has no file (have) and is not in the allow list.
func adrMissing(rel, text string, have map[int]bool, allow map[string][]int) []string {
	var out []string
	for n, l := range strings.Split(text, "\n") {
		for _, m := range adrRefRe.FindAllStringSubmatch(l, -1) {
			num, _ := strconv.Atoi(m[1])
			if have[num] {
				continue
			}
			ok := false
			for _, a := range allow[rel] {
				ok = ok || a == num
			}
			if !ok {
				out = append(out, fmt.Sprintf("%s:%d: ADR-%s has no file in docs/adr", rel, n+1, m[1]))
			}
		}
	}
	return out
}

var adrTextExt = map[string]bool{".md": true, ".go": true, ".yaml": true, ".yml": true, ".dart": true, ".rs": true, ".json": true,
	".txt": true, ".toml": true, ".sh": true, ".py": true, ".rego": true, ".patch": true}

// adrScanSkipped: a directory that is not read (build output, fixtures, the work of other sessions).
func adrScanSkipped(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		switch part {
		case ".git", "node_modules", "target", ".dart_tool", "testdata":
			return true
		}
	}
	return rel == ".claude/worktrees" || strings.HasPrefix(rel, ".claude/worktrees/") ||
		rel == ".claude/work" || strings.HasPrefix(rel, ".claude/work/")
}

// adrSelect: of the paths (slash, from the root) the text files that are read.
func adrSelect(rels []string) []string {
	var out []string
	for _, rel := range rels {
		if adrTextExt[filepath.Ext(rel)] && !adrScanSkipped(rel) {
			out = append(out, rel)
		}
	}
	return out
}

// adrScanFiles: the text files that the check reads. In a git work tree only the tracked files (an untracked file, such as a
// note of a session under .claude, is not part of the repository); without git, a walk that skips the directories above.
func adrScanFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	if out, err := cmd.Output(); err == nil {
		var rels []string
		for _, p := range strings.Split(string(out), "\x00") {
			if p != "" {
				rels = append(rels, p)
			}
		}
		return adrSelect(rels), nil
	}
	var rels []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && adrScanSkipped(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		rels = append(rels, rel)
		return nil
	})
	return adrSelect(rels), err
}

func TestAdrSelect(t *testing.T) {
	t.Parallel()
	got := adrSelect([]string{"docs/a.md", "x/testdata/b.md", ".claude/reviews/r.md", ".claude/work/w.md", "a.png", "cmd/x.go", "e/node_modules/z.json"})
	want := []string{"docs/a.md", ".claude/reviews/r.md", "cmd/x.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// the check still fails on a reference without a file, and passes on one with a file or on the allow list
func TestAdrMissing(t *testing.T) {
	t.Parallel()
	have := map[int]bool{1: true}
	if got := adrMissing("a.md", "see ADR-001\nand ADR-"+"999", have, nil); len(got) != 1 || !strings.HasPrefix(got[0], "a.md:2: ADR-"+"999") {
		t.Errorf("got %v", got)
	}
	if got := adrMissing("a.md", "ADR-"+"999", have, map[string][]int{"a.md": {999}}); len(got) != 0 {
		t.Errorf("allowed reference reported: %v", got)
	}
}
