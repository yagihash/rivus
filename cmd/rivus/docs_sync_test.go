package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/docsgen"
)

// TestDocsSync: the tables of the documents that copy a constant or a list of the code say the same as the code. The
// code is the source: a document is not edited by hand to follow it where a test can find the difference. The checks:
//
//   - the kinds of uncertainty (the Kind constants of internal/taint) are the rows of AGENT-GUIDE (the table of the kinds)
//     and of ARCHITECTURE 4.4;
//   - the first words of a `depth_limit` detail (the reasons) that the code writes are in the row of AGENT-GUIDE, and the
//     words that the two documents list exist in the code;
//   - the flags of `analyze` in AGENT-GUIDE section 4: the usual flags are the same set as the code's; a flag of the table
//     of the diagnostic flags is one of the code, with its default value;
//   - the keys of a rule file in MODELING.md section 6 are the `yaml:` keys of internal/model/model.go;
//   - the DESIGN-*.md files are all in docs/README.md, and the state at the head of each is one of the three of README
//     (案 / 一部実装 / 実装済み);
//   - the directories of internal/ (with a Go file) and of cmd/ are in the list of the packages of ARCHITECTURE 3, and the
//     internal/ paths of the list exist;
//   - the rows of the pass line (`pass_line` of bench/deps-targets.json, `make docs-gen WHAT=passline`) are named in
//     bench/passline-baseline.md, and the table is not copied there.
func TestDocsSync(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	read := func(p ...string) string {
		b, err := os.ReadFile(filepath.Join(append([]string{root}, p...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	// goSources is the text of the non-test Go files of the directories (each under internal/).
	goSources := func(dirs ...string) string {
		var sb strings.Builder
		for _, d := range dirs {
			files, _ := filepath.Glob(filepath.Join(root, "internal", d, "*.go"))
			for _, f := range files {
				if strings.HasSuffix(f, "_test.go") {
					continue
				}
				b, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				sb.Write(b)
				sb.WriteByte('\n')
			}
		}
		return sb.String()
	}
	// same reports the difference of two sets: what a is missing from b, and what b is missing from a.
	same := func(what, an string, a map[string]bool, bn string, b map[string]bool) {
		t.Helper()
		var onlyA, onlyB []string
		for k := range a {
			if !b[k] {
				onlyA = append(onlyA, k)
			}
		}
		for k := range b {
			if !a[k] {
				onlyB = append(onlyB, k)
			}
		}
		sort.Strings(onlyA)
		sort.Strings(onlyB)
		if len(onlyA) > 0 {
			t.Errorf("%s: in %s and not in %s: %v", what, an, bn, onlyA)
		}
		if len(onlyB) > 0 {
			t.Errorf("%s: in %s and not in %s: %v", what, bn, an, onlyB)
		}
	}
	// section is the lines from the heading that starts with head to the next heading of the same or a higher level.
	section := func(doc, head string) []string {
		lines := strings.Split(doc, "\n")
		var out []string
		in := false
		level := 0
		for _, l := range lines {
			if strings.HasPrefix(l, "#") {
				n := len(l) - len(strings.TrimLeft(l, "#"))
				if in && n <= level {
					break
				}
				if !in && strings.HasPrefix(l, head) {
					in, level = true, n
					continue
				}
			}
			if in {
				out = append(out, l)
			}
		}
		if len(out) == 0 {
			t.Fatalf("no section %q", head)
		}
		return out
	}
	cells := func(l string) []string {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "|") {
			return nil
		}
		l = strings.Trim(l, "|")
		l = strings.ReplaceAll(l, `\|`, "\x00")
		var out []string
		for _, c := range strings.Split(l, "|") {
			out = append(out, strings.TrimSpace(strings.ReplaceAll(c, "\x00", "|")))
		}
		return out
	}
	tick := regexp.MustCompile("`([^`]+)`")
	snake := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	// firstColumn is the words in backticks of the first cell of each row of the tables of the lines.
	firstColumn := func(lines []string) map[string]bool {
		out := map[string]bool{}
		for _, l := range lines {
			if c := cells(l); len(c) > 0 {
				for _, m := range tick.FindAllStringSubmatch(c[0], -1) {
					if snake.MatchString(m[1]) {
						out[m[1]] = true
					}
				}
			}
		}
		return out
	}

	agent := read("docs", "AGENT-GUIDE.md")
	arch := read("docs", "ARCHITECTURE.md")

	// 1. the kinds of uncertainty
	kinds := map[string]bool{}
	for _, m := range regexp.MustCompile(`\bKind\w+\s*=\s*"([a-z_]+)"`).FindAllStringSubmatch(goSources("taint"), -1) {
		kinds[m[1]] = true
	}
	if len(kinds) < 20 {
		t.Fatalf("the Kind constants of internal/taint: %d found", len(kinds))
	}
	same("the kinds, AGENT-GUIDE", "the code", kinds, "the document", firstColumn(section(agent, "### 不確実性の kind")))
	same("the kinds, ARCHITECTURE 4.4", "the code", kinds, "the document", firstColumn(section(arch, "### 4.4")))

	// 2. the reasons of depth_limit
	src := goSources("taint", "ir", "report")
	reasons := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?s)KindDepthLimit,\s*"([a-z_]+): `).FindAllStringSubmatch(src, -1) {
		reasons[m[1]] = true
	}
	var row string
	for _, l := range strings.Split(agent, "\n") {
		if strings.HasPrefix(l, "| `depth_limit` |") {
			row = l
		}
	}
	if row == "" || len(reasons) < 5 {
		t.Fatalf("the depth_limit row of AGENT-GUIDE (%d chars) or the reasons of the code (%d)", len(row), len(reasons))
	}
	wordRe := regexp.MustCompile("`([a-z_]+)`\\s*(?:\\(|/)")
	listed := func(text string) map[string]bool {
		out := map[string]bool{}
		for _, m := range wordRe.FindAllStringSubmatch(text, -1) {
			out[m[1]] = true
		}
		return out
	}
	if i := strings.Index(row, "`scc_coarsened`("); i > 0 {
		row = row[:i+len("`scc_coarsened`(")]
	}
	inDoc := listed(row)
	for r := range reasons {
		if !inDoc[r] {
			t.Errorf("depth_limit: the code writes the reason %q and the row of AGENT-GUIDE does not list it", r)
		}
	}
	archRow := ""
	for _, l := range section(arch, "### 4.4") {
		if strings.HasPrefix(l, "| `depth_limit` |") {
			archRow = l
		}
	}
	if i, j := strings.Index(archRow, "detail が理由を示す（"), strings.Index(archRow, "など）"); i > 0 && j > i {
		for _, m := range tick.FindAllStringSubmatch(archRow[i:j], -1) {
			inDoc[m[1]] = true
		}
	} else {
		t.Error("depth_limit: the row of ARCHITECTURE 4.4 has no list of the reasons")
	}
	for w := range inDoc {
		if !strings.Contains(src, `"`+w+`:`) && !strings.Contains(src, `"`+w+`"`) && !strings.Contains(src, w+`:`) {
			t.Errorf("depth_limit: the document lists the reason %q and the code does not write it", w)
		}
	}

	// 3. the flags of analyze
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	analyzeFlags(fs)
	sides := sidesOf(fs)
	usualCode, diagCode := map[string]bool{}, map[string]*flag.Flag{}
	fs.VisitAll(func(f *flag.Flag) {
		if sides.usual[f.Name] {
			usualCode[f.Name] = true
		} else {
			diagCode[f.Name] = f
		}
	})
	flagTick := regexp.MustCompile("`-([A-Za-z][A-Za-z0-9-]*)")
	usualDoc := map[string]bool{}
	for _, l := range section(agent, "### 共通のフラグ") {
		if c := cells(l); len(c) > 0 {
			for _, m := range flagTick.FindAllStringSubmatch(c[0], -1) {
				usualDoc[m[1]] = true
			}
		}
	}
	flagsDoc := section(agent, "## 4. フラグの一覧")
	for _, l := range flagsDoc {
		c := cells(l)
		if len(c) >= 2 && strings.Contains(c[0], "`analyze`") {
			for _, m := range flagTick.FindAllStringSubmatch(c[1], -1) {
				usualDoc[m[1]] = true
			}
		}
	}
	sameFlags := map[string]bool{}
	for k := range usualDoc {
		sameFlags[k] = true
	}
	same("the usual flags of analyze", "the code", usualCode, "AGENT-GUIDE 4", sameFlags)
	for _, l := range section(agent, "### 診断と調整のフラグ") {
		c := cells(l)
		if len(c) < 3 {
			continue
		}
		m := flagTick.FindStringSubmatch(c[0])
		if m == nil {
			continue
		}
		f := diagCode[m[1]]
		if f == nil {
			t.Errorf("the diagnostic flags: AGENT-GUIDE lists -%s, and analyze has no such flag (or it is a usual flag)", m[1])
			continue
		}
		if d := tick.FindStringSubmatch(c[2]); d != nil {
			def := d[1]
			if def == "off" {
				def = "false"
			}
			if d1, e1 := time.ParseDuration(def); e1 == nil {
				if d2, e2 := time.ParseDuration(f.DefValue); e2 == nil && d1 == d2 {
					continue
				}
			}
			if def != f.DefValue {
				t.Errorf("the diagnostic flags: -%s has the default %q in AGENT-GUIDE and %q in the code", m[1], d[1], f.DefValue)
			}
		}
	}

	// 4. the keys of a rule file: the table of the top level is the keys of RuleSet; a key of the table of the rows is a key of the loader
	// (the table is a summary: it does not list every key of the rows that are inside a row).
	model := read("internal", "model", "model.go")
	keyRe := regexp.MustCompile(`yaml:"([a-z_]+)[,"]`)
	allKeys := map[string]bool{}
	for _, m := range keyRe.FindAllStringSubmatch(model, -1) {
		allKeys[m[1]] = true
	}
	ruleSet := model[strings.Index(model, "type RuleSet struct"):]
	ruleSet = ruleSet[:strings.Index(ruleSet, "\n}")]
	topKeys := map[string]bool{}
	for _, m := range keyRe.FindAllStringSubmatch(ruleSet, -1) {
		topKeys[m[1]] = true
	}
	var topDoc, rowDoc []string
	rowPart := false
	for _, l := range section(read("docs", "MODELING.md"), "## 6. Keys of a rule file") {
		if strings.HasPrefix(l, "**A row**") {
			rowPart = true
		}
		if rowPart {
			rowDoc = append(rowDoc, l)
		} else {
			topDoc = append(topDoc, l)
		}
	}
	same("the keys at the top of a rule file", "internal/model/model.go (RuleSet)", topKeys, "MODELING.md 6", firstColumn(topDoc))
	for k := range firstColumn(rowDoc) {
		if !allKeys[k] {
			t.Errorf("the keys of a row: MODELING.md 6 lists %q, and the loader (internal/model/model.go) has no such key", k)
		}
	}

	// 5. the rows of the pass line
	rows, err := docsgen.LoadPassLine(filepath.Join(root, "bench", "deps-targets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Error("bench/deps-targets.json has no target with pass_line")
	}
	table := docsgen.PassLineTable(rows)
	for _, r := range rows {
		if !strings.Contains(table, "| "+r.Line+" | `"+r.Target+"` |") {
			t.Errorf("the table of docs-gen passline has no row for %s", r.Target)
		}
	}
	baseline := read("bench", "passline-baseline.md")
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.Line] {
			t.Errorf("pass_line %q is on two targets", r.Line)
		}
		seen[r.Line] = true
		if !strings.Contains(baseline, r.Target) && !strings.Contains(baseline, strings.SplitN(r.Target, "-", 2)[0]) {
			t.Errorf("the target %s (pass line %s) is not named in bench/passline-baseline.md", r.Target, r.Line)
		}
	}
	if !strings.Contains(baseline, "docs-gen WHAT=passline") {
		t.Error("bench/passline-baseline.md does not point to `make docs-gen WHAT=passline`")
	}

	// 6. the DESIGN documents: each is in docs/README.md, and its state is one of the three
	readme := read("docs", "README.md")
	designs, _ := filepath.Glob(filepath.Join(root, "docs", "DESIGN-*.md"))
	if len(designs) < 10 {
		t.Fatalf("docs/DESIGN-*.md: %d found", len(designs))
	}
	stateRe := regexp.MustCompile(`(?:状態|Status):\s*\**\s*(案|一部実装|実装済み)`)
	for _, d := range designs {
		name := filepath.Base(d)
		if !strings.Contains(readme, "]("+name+")") {
			t.Errorf("docs/README.md does not list docs/%s", name)
		}
		lines := strings.Split(read("docs", name), "\n")
		if len(lines) > 12 {
			lines = lines[:12]
		}
		if !stateRe.MatchString(strings.Join(lines, "\n")) {
			t.Errorf("docs/%s: no state (案 / 一部実装 / 実装済み, after `状態:`) in the first lines", name)
		}
	}

	// 7. the directories of the code are in the list of the packages (ARCHITECTURE 3)
	listedDirs := map[string]bool{}
	inInternal := false
	topRe := regexp.MustCompile(`^(\S+)/(\s|$)`)
	subRe := regexp.MustCompile(`^( {2}| {4})(\S+)/(\s|$)`)
	top := ""
	for _, l := range section(arch, "## 3.") {
		if m := topRe.FindStringSubmatch(l); m != nil {
			inInternal = m[1] == "internal"
			if strings.HasPrefix(m[1], "cmd/") {
				listedDirs[m[1]] = true
			}
			continue
		}
		if m := subRe.FindStringSubmatch(l); m != nil && inInternal {
			if len(m[1]) == 2 {
				top = m[2]
				listedDirs["internal/"+top] = true
			} else if top != "" {
				listedDirs["internal/"+top+"/"+m[2]] = true
			}
		}
	}
	hasGo := map[string]bool{}
	for _, base := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, base), func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				if e.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".go") {
				rel, _ := filepath.Rel(root, filepath.Dir(p))
				hasGo[filepath.ToSlash(rel)] = true
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for d := range hasGo {
		if !listedDirs[d] {
			t.Errorf("ARCHITECTURE 3 does not list the directory %s (it has Go files)", d)
		}
	}
	for d := range listedDirs {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(d))); err != nil {
			t.Errorf("ARCHITECTURE 3 lists %s, and there is no such directory", d)
		}
	}
}
