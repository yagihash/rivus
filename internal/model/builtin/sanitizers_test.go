package builtin_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// A sanitizer of the built-in model is listed in testdata/sanitizers-reviewed.txt with what kind of
// symbol it is. A sanitizer says only that its result is clean, so what a call through it reaches in the
// code of the project (an implementation of the interface) is read, with its result dropped (ADR-086).
// A row that is added without being read fails here, and so does one that was taken out and is still
// listed. The content of the rows is read too: the third column is a hash of what the rows say (drops, adds, neutralises,
// flows, match, max_implementations, allow_sanitizer_expansion; not the provenance), so a change of `match:` or of
// `allow_sanitizer_expansion` fails here until the new content is read and the hash is written (the failure prints
// it). A row on a method of an interface or a trait is the case that the rule is for: it is counted.
func TestSanitizersAreReviewed(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	rules := model.Merge(sets...)
	data, err := os.ReadFile("testdata/sanitizers-reviewed.txt")
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]string{}
	hashes := map[string]string{}
	comments := map[string]string{}
	categories := map[string]bool{}
	for _, rs := range sets {
		for _, sk := range rs.Sinks {
			categories[sk.Category] = true
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		line, comment, _ := strings.Cut(line, " #") // a `#` inside a Dart symbol is no comment
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 3 || !strings.HasPrefix(f[2], "sha256:") {
			t.Fatalf("testdata/sanitizers-reviewed.txt: want `symbol kind sha256:<hash of the rows>`, got %q", line)
		}
		switch f[1] {
		case "function", "static", "class-method", "interface":
		default:
			t.Fatalf("testdata/sanitizers-reviewed.txt: %s: kind %q is none of function, static, class-method, interface", f[0], f[1])
		}
		listed[f[0]] = f[1]
		comments[f[0]] = strings.TrimSpace(comment)
		hashes[f[0]] = strings.TrimPrefix(f[2], "sha256:")
	}
	var have []string
	for sym := range rules.Sanitizers {
		have = append(have, sym)
	}
	sort.Strings(have)
	for _, sym := range have {
		if _, ok := listed[sym]; !ok {
			t.Errorf("the sanitizer %s is not in testdata/sanitizers-reviewed.txt: read what kind of symbol it is (a function, a static method, a method a subclass may override, a method of an interface or a trait) and list it", sym)
		}
		// What a sanitizer neutralises is written in every builtin row (ADR-114): a row without it hides the value from the sinks of
		// every category, and a row with an empty list says why in the comment of the reviewed file.
		for _, row := range rules.Sanitizers[sym] {
			switch {
			case row.Neutralises == nil:
				t.Errorf("the sanitizer %s has no neutralises: write the categories of the sinks that it makes the value safe for (an empty list when none is certain, with the reason as a comment in testdata/sanitizers-reviewed.txt)", sym)
			case len(*row.Neutralises) == 0 && comments[sym] == "":
				t.Errorf("the sanitizer %s has an empty neutralises: write the reason after `#` in testdata/sanitizers-reviewed.txt", sym)
			default:
				for _, c := range *row.Neutralises {
					if !categories[c] {
						t.Errorf("the sanitizer %s neutralises %q, which is the category of no sink of the built-in model", sym, c)
					}
				}
			}
		}
		if want, ok := hashes[sym]; ok {
			if got := rowsHash(rules.Sanitizers[sym]); got != want {
				t.Errorf("the rows of the sanitizer %s changed (their hash is sha256:%s, the file has sha256:%s): read what they say now (match, allow_sanitizer_expansion, flows) and write the new hash", sym, got, want)
			}
		}
		delete(listed, sym)
	}
	for sym := range listed {
		t.Errorf("testdata/sanitizers-reviewed.txt lists %s, which is no sanitizer of the built-in model", sym)
	}
}

// rowsHash is a hash of what the rows of one symbol say: every field but the provenance (who decided it and why).
func rowsHash(rows []model.Sanitizer) string {
	h := sha256.New()
	for _, r := range rows {
		var n []string
		if r.Neutralises != nil {
			n = append([]string{}, *r.Neutralises...)
		}
		fmt.Fprintf(h, "%q|%q|%q|%+v|%q|%d|%v|%v|%v\n", r.Symbol, r.Drops, r.Adds, r.Flows, r.Match, r.MaxImplementations, r.AllowSanitizerExpansion, r.Neutralises != nil, n)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
