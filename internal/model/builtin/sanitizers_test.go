package builtin_test

import (
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
// listed. A row on a method of an interface or a trait is the case that the rule is for: it is counted.
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
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		line, _, _ = strings.Cut(line, " #") // a `#` inside a Dart symbol is no comment
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 2 {
			t.Fatalf("testdata/sanitizers-reviewed.txt: want `symbol kind`, got %q", line)
		}
		switch f[1] {
		case "function", "static", "class-method", "interface":
		default:
			t.Fatalf("testdata/sanitizers-reviewed.txt: %s: kind %q is none of function, static, class-method, interface", f[0], f[1])
		}
		listed[f[0]] = f[1]
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
		delete(listed, sym)
	}
	for sym := range listed {
		t.Errorf("testdata/sanitizers-reviewed.txt lists %s, which is no sanitizer of the built-in model", sym)
	}
}
