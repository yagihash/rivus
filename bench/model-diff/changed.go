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

// changedPackages lists the packages whose flows or sinks rows differ from the ones at the commit `base`, in the built-in model
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
		for k, v := range b {
			if a[k] != v {
				set[lang+" "+SymbolPackage(lang, strings.SplitN(k, " ", 2)[1])] = true
			}
		}
		for k, v := range a {
			if b[k] != v {
				set[lang+" "+SymbolPackage(lang, strings.SplitN(k, " ", 2)[1])] = true
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

// rowsOf maps "<kind> <symbol>" to the row as JSON (the flows rows and the sinks rows).
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
	}
	for _, s := range rs.Sinks {
		j, _ := json.Marshal(s)
		m["sink "+s.Symbol] = string(j)
	}
	return m
}
