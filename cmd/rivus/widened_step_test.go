package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
)

// ステップの単位の注記(#186、ADR-118 の追記): 畳まれた出力(24 要素を超えて、パラメータの部分を全体にまとめた出力)の、
// 全体になった要素を適用したステップに、depth_limit の注記(detail が out_widened: で始まる)が付く。畳んでいない出力には付かない。
// finding の uncertainties と confidence は、この注記で変わらない。付くステップと付かないステップの集合は、関数の解析の順序にも、
// ワーカーの数にも依らない(ADR-118 の (ii))。

// widenedModule writes a module where Pack returns n fields of one struct (n > 24: the output of Pack is widened) and
// the order of the declarations is order (a permutation of the names below), one function per file when split.
func widenedModule(t *testing.T, n int, order []string, split bool) string {
	t.Helper()
	dir := t.TempDir()
	var fields, elems strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&fields, "\tF%d string\n", i)
		if i > 0 {
			elems.WriteString(", ")
		}
		fmt.Fprintf(&elems, "p.F%d", i)
	}
	decl := map[string]string{
		"T":    "type T struct {\n" + fields.String() + "}\n",
		"Src":  "func Src() string { return \"x\" }\n",
		"Pack": "func Pack(p *T) []string { return []string{" + elems.String() + "} }\n",
		"Wrap": "func Wrap(p *T) []string { return Pack(p) }\n",
		"main": "func main() {\n\tt := &T{F0: Src()}\n\tb := Wrap(t)\n\tos.Open(b[0])\n}\n",
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module w\n\ngo 1.22\n")
	write("rules.yaml", "ruleset: w@1\nsources:\n  - symbol: w.Src\n    labels: [user_input]\nsinks:\n  - symbol: os.Open\n    args: [0]\n    category: path_traversal\n")
	if split {
		for i, name := range order {
			write(fmt.Sprintf("f%02d_%s.go", i, name), "package main\n\nimport \"os\"\n\nvar _ = os.Open\n\n"+decl[name])
		}
		return dir
	}
	var all strings.Builder
	all.WriteString("package main\n\nimport \"os\"\n\n")
	for _, name := range order {
		all.WriteString(decl[name] + "\n")
	}
	write("main.go", all.String())
	return dir
}

// stepNotes is the steps of the finding paths, one line for each: the function, what it did, and whether it has the note.
// The set of the lines is what must not depend on the order (the positions are not in it: they follow the files).
func stepNotes(t *testing.T, out []byte) (lines []string, rep report.Report) {
	t.Helper()
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range rep.Findings {
		for _, p := range f.Paths {
			for _, st := range p.Steps {
				noted := false
				for _, n := range st.Notes {
					if n.Kind == "depth_limit" && strings.HasPrefix(n.Detail, "out_widened:") {
						noted = true
					}
				}
				l := fmt.Sprintf("%s | %s | %v", st.Func, st.Desc, noted)
				if !seen[l] {
					seen[l] = true
					lines = append(lines, l)
				}
			}
		}
	}
	sort.Strings(lines)
	return lines, rep
}

func countNoted(lines []string) int {
	n := 0
	for _, l := range lines {
		if strings.HasSuffix(l, "| true") {
			n++
		}
	}
	return n
}

func analyzeModule(t *testing.T, dir string, workers string) []byte {
	t.Helper()
	return analyzeWith(t, dir, "", "-workers", workers, "-rules", "rules.yaml", "./...")
}

func TestWidenedOutputStepNote(t *testing.T) {
	base := []string{"T", "Src", "Pack", "Wrap", "main"}

	// (a) 30 fields: the hop that applies the summary of Pack (in Wrap) and the one that applies Wrap (in main) have the note
	lines, rep := stepNotes(t, analyzeModule(t, widenedModule(t, 30, base, false), "1"))
	if len(rep.Findings) == 0 {
		t.Fatalf("no finding\n%v", lines)
	}
	if countNoted(lines) == 0 {
		t.Errorf("a widened output, but no hop has the out_widened note:\n%s", strings.Join(lines, "\n"))
	}
	for _, f := range rep.Findings {
		// the note is on a hop of the path, so it is in the uncertainties of the finding too, and the finding is to be reviewed
		found := false
		for _, u := range f.Uncertainties {
			if u.Kind == "depth_limit" && strings.HasPrefix(u.Detail, "out_widened:") {
				found = true
			}
		}
		if !found {
			t.Errorf("finding %s: the out_widened note is not in its uncertainties", f.ID)
		}
		if f.Confidence != "requires_review" {
			t.Errorf("finding %s: confidence %q", f.ID, f.Confidence)
		}
	}

	// (b) 10 fields: nothing is widened, no hop has the note
	lines, rep = stepNotes(t, analyzeModule(t, widenedModule(t, 10, base, false), "1"))
	if len(rep.Findings) == 0 {
		t.Fatal("no finding with 10 fields")
	}
	if n := countNoted(lines); n != 0 {
		t.Errorf("not widened, but %d hops have the note:\n%s", n, strings.Join(lines, "\n"))
	}
}

// (c) the order of the declarations, the order of the files, and -workers: the same hops have the note.
func TestWidenedOutputStepNoteOrder(t *testing.T) {
	orders := [][]string{
		{"T", "Src", "Pack", "Wrap", "main"},
		{"main", "Wrap", "Pack", "Src", "T"},
		{"Wrap", "main", "T", "Pack", "Src"},
		{"Pack", "T", "main", "Src", "Wrap"},
	}
	var first []string
	var firstHow string
	for oi, order := range orders {
		for _, split := range []bool{false, true} {
			for _, w := range []string{"1", "4"} {
				lines, _ := stepNotes(t, analyzeModule(t, widenedModule(t, 30, order, split), w))
				how := fmt.Sprintf("order %d split=%v -workers %s", oi, split, w)
				if first == nil {
					if countNoted(lines) == 0 {
						t.Fatalf("%s: no hop has the note", how)
					}
					first, firstHow = lines, how
					continue
				}
				if strings.Join(first, "\n") != strings.Join(lines, "\n") {
					t.Fatalf("%s and %s differ:\n%s\n--\n%s", firstHow, how, strings.Join(first, "\n"), strings.Join(lines, "\n"))
				}
			}
		}
	}
}

// A finding that had no uncertainty before is to be reviewed once its path goes through the widened output: the hop has
// the note, the finding has it in its uncertainties, and confidence is requires_review (the same as access_path_truncated).
// With fewer fields (nothing widened) it is no_uncertainty_on_path.
func TestWidenedOutputConfidence(t *testing.T) {
	run := func(n int) *taint.Finding {
		dir := t.TempDir()
		var fields strings.Builder
		var cat []string
		for i := 0; i < n; i++ {
			fmt.Fprintf(&fields, "\tF%d string\n", i)
			cat = append(cat, fmt.Sprintf("p.F%d", i))
		}
		src := "package main\n\nimport \"os\"\n\ntype T struct {\n" + fields.String() + "}\n\nfunc Src() string { return \"x\" }\n\n" +
			"func Cat(p *T) string { return " + strings.Join(cat, " + ") + " }\n\nfunc Wrap(p *T) string { return Cat(p) }\n\n" +
			"func main() {\n\tt := &T{F0: Src()}\n\tos.Open(Wrap(t))\n}\n"
		for name, body := range map[string]string{
			"main.go":    src,
			"go.mod":     "module w\n\ngo 1.22\n",
			"rules.yaml": "ruleset: w@1\nsources:\n  - symbol: w.Src\n    labels: [user_input]\nsinks:\n  - symbol: os.Open\n    args: [0]\n    category: path_traversal\n",
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, rep := stepNotes(t, analyzeModule(t, dir, "1"))
		if len(rep.Findings) != 1 {
			t.Fatalf("%d fields: %d findings", n, len(rep.Findings))
		}
		return rep.Findings[0]
	}
	small, big := run(10), run(30)
	if small.Confidence != "no_uncertainty_on_path" || len(small.Uncertainties) != 0 {
		t.Errorf("10 fields: confidence %q, %d uncertainties; want no_uncertainty_on_path, none", small.Confidence, len(small.Uncertainties))
	}
	if big.Confidence != "requires_review" {
		t.Errorf("30 fields: confidence %q, want requires_review", big.Confidence)
	}
	found := false
	for _, u := range big.Uncertainties {
		found = found || (u.Kind == "depth_limit" && strings.HasPrefix(u.Detail, "out_widened:"))
	}
	if !found {
		t.Errorf("30 fields: no out_widened in the uncertainties: %+v", big.Uncertainties)
	}
	if small.ID != big.ID {
		t.Logf("ids: %s %s", small.ID, big.ID)
	}
}
