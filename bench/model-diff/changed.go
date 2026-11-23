package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/yagihash/rivus/internal/model"
)

// changedPackages lists the packages whose rows (flows, pure, sinks, sanitizers, sources) or whose `body_replaced_packages` declaration differ from the ones at the commit `base`, in the built-in model
// files: what a pull request that changes a model has to check ("go <pkg>" / "dart <pkg>", one per line; Rust is not checked).
func changedPackages(repo, base string) ([]string, error) {
	c := exec.Command("git", "diff", "--name-only", base, "--", "internal/model/builtin")
	c.Dir = repo
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %v", err)
	}
	set := map[string]bool{}
	for _, f := range strings.Fields(string(out)) {
		if !strings.HasSuffix(f, ".yaml") {
			continue
		}
		lang := strings.SplitN(filepath.Base(f), "-", 2)[0]
		if lang != "go" && lang != "dart" {
			continue
		}
		show := exec.Command("git", "show", base+":"+f)
		show.Dir = repo
		old, _ := show.Output() // a new file: no rows before
		now, _ := os.ReadFile(filepath.Join(repo, f))
		a, b := rowsOf(old), rowsOf(now)
		mark := func(k string) {
			if strings.HasPrefix(k, "declared ") {
				// a package that the file declares as one whose body the models replace: all of its rows are checked
				set[lang+" "+strings.TrimSuffix(rowName(k), "/...")] = true
				return
			}
			set[lang+" "+SymbolPackage(lang, rowName(k))] = true
		}
		for k, v := range b {
			if a[k] != v {
				mark(k)
			}
		}
		for k, v := range a {
			if b[k] != v {
				mark(k)
			}
		}
	}
	var res []string
	for k := range set {
		if !strings.HasSuffix(k, " ") {
			res = append(res, k)
		}
	}
	sort.Strings(res)
	return res, nil
}

// rowName is the symbol (or field, or type) of a key of rowsOf: the last word.
func rowName(k string) string { return k[strings.LastIndexByte(k, ' ')+1:] }

// rowsOf maps "<kind> <symbol>" to the row as JSON: the flows rows, the pure rows, the sinks, the sanitizers and the sources (a source
// without a symbol is named by its field or its type).
func rowsOf(text []byte) map[string]string {
	var rs model.RuleSet
	m := map[string]string{}
	if yaml.Unmarshal(text, &rs) != nil {
		return m
	}
	for _, p := range rs.Propagators {
		if len(p.Flows) > 0 {
			j, _ := json.Marshal(p.Flows)
			m["flows "+p.Symbol] = string(j)
		}
		if p.Pure {
			m["pure "+p.Symbol] = "pure"
		}
	}
	for _, s := range rs.Sinks {
		j, _ := json.Marshal(s)
		m["sink "+s.Symbol] = string(j)
	}
	for _, d := range rs.BodyReplaced {
		j, _ := json.Marshal(d)
		m["declared "+d.Package] = string(j)
	}
	for _, s := range rs.Sanitizers {
		j, _ := json.Marshal(s)
		m["sanitizer "+s.Symbol] = string(j)
	}
	for _, s := range rs.Sources {
		name := s.Symbol
		if name == "" {
			name = strings.TrimPrefix(s.Field+s.Type, "*")
		}
		if name == "" {
			continue
		}
		j, _ := json.Marshal(s)
		m["source "+s.Kind+" "+name] = string(j)
	}
	return m
}

// addedReviewed lists the `miss-reviewed` notes that the built-in model files have and the commit `base` does not, one line each:
// `miss-reviewed-added <file> <symbol>@<position> <kind>`. A pull request that adds one gets a notice from the job (it does not fail).
func addedReviewed(repo, base string) ([]string, error) {
	c := exec.Command("git", "diff", "--name-only", base, "--", "internal/model/builtin")
	c.Dir = repo
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %v", err)
	}
	var res []string
	for _, f := range strings.Fields(string(out)) {
		if !strings.HasSuffix(f, ".yaml") {
			continue
		}
		show := exec.Command("git", "show", base+":"+f)
		show.Dir = repo
		old, _ := show.Output()
		now, _ := os.ReadFile(filepath.Join(repo, f))
		before := map[string]bool{}
		on, _ := ParseNotes(f, string(old))
		for _, n := range on {
			if n.Kind == "miss-reviewed" {
				before[n.Symbol] = true
			}
		}
		nn, _ := ParseNotes(f, string(now))
		for _, n := range nn {
			if n.Kind == "miss-reviewed" && !before[n.Symbol] {
				k, _ := ReviewKind(n)
				res = append(res, fmt.Sprintf("miss-reviewed-added %s %s %s", f, n.Symbol, k))
			}
		}
	}
	sort.Strings(res)
	return res, nil
}
