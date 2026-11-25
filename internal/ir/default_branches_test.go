package ir

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// An extractor must not drop a kind of syntax that it does not know without a word: `unsupported_construct` that counts 0 says nothing when the
// branch for the other kinds passes over the node. This test reads the sources of the Rust and the Dart extractors and sorts every branch for
// "the others" (Rust: `_ =>` and `other =>`, Dart: `default:`) into one of four:
//
//   - it reports: the body says `unsupported` (Rust `self.unsupported(`, Dart `'unsupported'`);
//   - `default-ok: <reason>`: a comment on the line, just above it, or just below it (inside the body) says why passing over is right, or what
//     reports it elsewhere. The reason is required. Only for a branch that was read and found right. A branch that is not about a kind of
//     syntax (a type, a string, an argument) says so there (`not a syntax kind: ...`);
//   - the list of the branches not examined (default_branches_todo.txt): a branch that does drop a kind, or that cannot be judged yet. The list
//     only gets shorter: an entry that is no longer needed (fixed, now `default-ok`, or gone) fails here, so that it is removed;
//   - none of these: a new branch, which fails.
//
// What this does not see: a chain of `if (x is T) ... else if (x is U)` in Dart, and a Dart `switch` that has no `default:`. Those were read
// once (patterns.dart: the `switch` is over a sealed class, so that the compiler rejects a kind that is not handled; the chain in the map
// pattern covers the two kinds that a map pattern element has; lower.dart: the chain of the kinds of a function body ends where there is no
// code to read; the chain of the members of a switch has the default member taken out before). A new one of that kind is a matter for the review.
//
// The Go extractor (internal/frontend) has one place that drops an instruction of go/ssa: the `default:` of the switch in goir.go that emits
// OpUnsupported. The other `default:` there narrow a search (the instructions that are looked at, the kinds of a callee) and do not skip code.

// Rust: the rules of the files that are read; Dart: the same.
type branchRule struct {
	dir     []string
	file    string
	branch  *regexp.Regexp
	reports *regexp.Regexp
}

var branchRules = []branchRule{
	{
		dir:     []string{"extractor/rust/src"},
		file:    ".rs",
		branch:  regexp.MustCompile(`^\s*(_|other)( if .*)? =>`),
		reports: regexp.MustCompile(`\.unsupported\(`),
	},
	{
		dir:     []string{"extractor/dart/lib", "extractor/dart/bin"},
		file:    ".dart",
		branch:  regexp.MustCompile(`^\s*default:`),
		reports: regexp.MustCompile(`'unsupported'`),
	},
}

var defaultOK = regexp.MustCompile(`//\s*default-ok:(.*)$`)

type defaultBranch struct {
	file string // from the root of the repository, with `/`
	line int    // 1-based; for the message only
	key  string // the branch and the line after it, without the space in them
	nth  int    // which of the branches that have the same key in the file (1-based)
	// the class
	reports bool
	ok      bool
	reason  string // of default-ok, trimmed
	hasOK   bool   // the comment is there (the reason may be empty)
}

func squeeze(s string) string { return strings.Join(strings.Fields(s), " ") }

func indentOf(s string) int { return len(s) - len(strings.TrimLeft(s, " \t")) }

// branchesOf finds the branches in the lines of one file and classifies them.
func branchesOf(file string, lines []string, r branchRule) []defaultBranch {
	var out []defaultBranch
	seen := map[string]int{}
	for i, l := range lines {
		if !r.branch.MatchString(l) {
			continue
		}
		next := ""
		if i+1 < len(lines) {
			next = lines[i+1]
		}
		b := defaultBranch{file: file, line: i + 1, key: squeeze(l) + " ⏎ " + squeeze(next)}
		seen[b.key]++
		b.nth = seen[b.key]
		// the body: the line itself, and the lines under it that are indented more (at most 12)
		ind := indentOf(l)
		body := []string{l}
		for j := i + 1; j < len(lines) && j <= i+12; j++ {
			if strings.TrimSpace(lines[j]) == "" || indentOf(lines[j]) <= ind {
				break
			}
			body = append(body, lines[j])
		}
		for _, bl := range body {
			if r.reports.MatchString(bl) {
				b.reports = true
			}
		}
		// the comments: on the line, the comment lines right above, the comment lines right under
		var cands []string
		cands = append(cands, l)
		for j := i - 1; j >= 0 && strings.HasPrefix(strings.TrimSpace(lines[j]), "//"); j-- {
			cands = append(cands, lines[j])
		}
		for j := i + 1; j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "//"); j++ {
			cands = append(cands, lines[j])
		}
		for _, c := range cands {
			if m := defaultOK.FindStringSubmatch(c); m != nil {
				b.hasOK = true
				b.reason = strings.TrimSpace(m[1])
				b.ok = b.reason != ""
				break
			}
		}
		out = append(out, b)
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func allDefaultBranches(t *testing.T) []defaultBranch {
	t.Helper()
	root := repoRoot(t)
	var all []defaultBranch
	for _, r := range branchRules {
		for _, d := range r.dir {
			ents, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(d)))
			if err != nil {
				t.Fatalf("%s: %v", d, err)
			}
			n := 0
			for _, e := range ents {
				if e.IsDir() || !strings.HasSuffix(e.Name(), r.file) {
					continue
				}
				n++
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(d), e.Name()))
				if err != nil {
					t.Fatal(err)
				}
				all = append(all, branchesOf(d+"/"+e.Name(), strings.Split(string(data), "\n"), r)...)
			}
			if n == 0 {
				t.Fatalf("%s: no %s file (the test would see nothing)", d, r.file)
			}
		}
	}
	return all
}

type todoEntry struct {
	file, key string
	nth       int
	reason    string
	line      int // in the list file
}

// todoFile: one entry a line, `file<TAB>nth<TAB>branch ⏎ next line<TAB>reason`; `#` starts a comment line.
func readTodo(t *testing.T) []todoEntry {
	t.Helper()
	data, err := os.ReadFile("default_branches_todo.txt")
	if err != nil {
		t.Fatal(err)
	}
	var out []todoEntry
	for i, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
			continue
		}
		f := strings.Split(l, "\t")
		if len(f) != 4 {
			t.Fatalf("default_branches_todo.txt:%d: want file, nth, key and reason separated by tabs", i+1)
		}
		n, err := strconv.Atoi(f[1])
		if err != nil || n < 1 {
			t.Fatalf("default_branches_todo.txt:%d: nth is not a number", i+1)
		}
		if strings.TrimSpace(f[3]) == "" {
			t.Fatalf("default_branches_todo.txt:%d: the reason is empty", i+1)
		}
		out = append(out, todoEntry{file: f[0], nth: n, key: f[2], reason: f[3], line: i + 1})
	}
	return out
}

func TestDefaultBranchesAreAccountedFor(t *testing.T) {
	all := allDefaultBranches(t)
	todo := readTodo(t)
	listed := map[string]*todoEntry{}
	for i := range todo {
		e := &todo[i]
		id := e.file + "\x00" + strconv.Itoa(e.nth) + "\x00" + e.key
		if listed[id] != nil {
			t.Errorf("default_branches_todo.txt:%d: the entry is there twice", e.line)
		}
		listed[id] = e
	}
	var nReports, nOK, nTodo int
	used := map[string]bool{}
	for _, b := range all {
		id := b.file + "\x00" + strconv.Itoa(b.nth) + "\x00" + b.key
		e := listed[id]
		switch {
		case b.hasOK && !b.ok:
			t.Errorf("%s:%d: `default-ok:` has no reason", b.file, b.line)
		case b.reports:
			nReports++
			if e != nil {
				t.Errorf("default_branches_todo.txt:%d: %s:%d reports `unsupported` now: remove the entry", e.line, b.file, b.line)
			}
		case b.ok:
			nOK++
			if e != nil {
				t.Errorf("default_branches_todo.txt:%d: %s:%d is `default-ok` now: remove the entry", e.line, b.file, b.line)
			}
		case e != nil:
			nTodo++
			used[id] = true
		default:
			t.Errorf("%s:%d: a branch for the other cases that does not report `unsupported` and has no reason (`// default-ok: <reason>`): %s\n"+
				"\tReport `unsupported` for a kind of syntax that is not known, or write why passing over is right. If it does drop a kind and is not fixed yet,\n"+
				"\tit goes to internal/ir/default_branches_todo.txt (the list only gets shorter):\n\t%s\t%d\t%s\t<one sentence: which kind is dropped, no input>",
				b.file, b.line, strings.Split(b.key, " ⏎ ")[0], b.file, b.nth, b.key)
		}
	}
	for id, e := range listed {
		if !used[id] {
			// not reported above as a branch that is now classified: the branch is gone, or its text changed
			found := false
			for _, b := range all {
				if b.file+"\x00"+strconv.Itoa(b.nth)+"\x00"+b.key == id {
					found = true
				}
			}
			if !found {
				t.Errorf("default_branches_todo.txt:%d: no such branch in %s any more (fixed, moved or changed): remove the entry", e.line, e.file)
			}
		}
	}
	t.Logf("default branches: %d report unsupported, %d default-ok, %d in the list of those not examined (default_branches_todo.txt), %d in all", nReports, nOK, nTodo, len(all))
}

// The tests of the test: the classes, on text that is made here.
func TestDefaultBranchClasses(t *testing.T) {
	rust := branchRules[0]
	src := strings.Split(`fn f() {
    match k {
        A => 1,
        _ => {
            self.unsupported("x");
        }
    }
    match k {
        // default-ok: not a syntax kind: a string
        _ => 2,
    }
    match k {
        _ => 3, // default-ok:
    }
    match k {
        other if cond => {
            let _ = other;
        }
    }
}`, "\n")
	got := branchesOf("x.rs", src, rust)
	if len(got) != 4 {
		t.Fatalf("branches = %d, want 4", len(got))
	}
	if !got[0].reports || got[0].ok {
		t.Errorf("the branch that reports: %+v", got[0])
	}
	if !got[1].ok || got[1].reason != "not a syntax kind: a string" {
		t.Errorf("the branch with a reason: %+v", got[1])
	}
	if !got[2].hasOK || got[2].ok {
		t.Errorf("the empty reason must not count: %+v", got[2])
	}
	if got[3].reports || got[3].hasOK {
		t.Errorf("a new branch is none of them: %+v", got[3])
	}
	dart := branchRules[1]
	d := branchesOf("x.dart", strings.Split("switch (x) {\n  case A():\n    break;\n  default:\n    f.emit('unsupported', at: a);\n}\nswitch (y) {\n  default:\n    // default-ok: only the text\n    continue;\n}\n", "\n"), dart)
	if len(d) != 2 || !d[0].reports || !d[1].ok {
		t.Errorf("dart: %+v", d)
	}
}
