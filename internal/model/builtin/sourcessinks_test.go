package builtin_test

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// WithoutSourcesSinks leaves out the sources and the sinks of the built-in files, and nothing else: the indexes
// that Merge makes from the sources and the sinks are empty, the ones made from the other sections are not.
func TestWithoutSourcesSinks(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	var before, srcs, snks int
	for _, rs := range sets {
		before += len(rs.Propagators)
		srcs += len(rs.Sources)
		snks += len(rs.Sinks)
	}
	if srcs == 0 || snks == 0 {
		t.Fatalf("the built-in files have %d sources and %d sinks", srcs, snks)
	}
	var cut []*model.RuleSet
	for _, rs := range sets {
		cut = append(cut, rs.WithoutSourcesSinks())
	}
	// the sets that were given are not changed
	var srcs2, snks2 int
	for _, rs := range sets {
		srcs2 += len(rs.Sources)
		snks2 += len(rs.Sinks)
	}
	if srcs2 != srcs || snks2 != snks {
		t.Errorf("the original sets changed: sources %d -> %d, sinks %d -> %d", srcs, srcs2, snks, snks2)
	}
	r := model.Merge(cut...)
	if len(r.Sinks) != 0 || len(r.CallSources) != 0 || len(r.ParamTypes) != 0 || len(r.Fields) != 0 || len(r.CallbackParams) != 0 || len(r.ParamFuncs) != 0 {
		t.Errorf("sources or sinks are left: sinks %d, call %d, param_type %d, fields %d, callback %d, param %d",
			len(r.Sinks), len(r.CallSources), len(r.ParamTypes), len(r.Fields), len(r.CallbackParams), len(r.ParamFuncs))
	}
	if len(r.Propagators) == 0 || len(r.Sanitizers) == 0 {
		t.Errorf("the other sections are gone: propagators %d, sanitizers %d", len(r.Propagators), len(r.Sanitizers))
	}
	var after int
	for _, rs := range cut {
		after += len(rs.Propagators)
		if !rs.SourcesSinksLeftOut {
			t.Errorf("%s: not marked", rs.Name)
		}
	}
	if after != before {
		t.Errorf("propagators %d -> %d", before, after)
	}
}

// The sources of Rust and of Dart are left out as the ones of Go are (by the names of the rule sets, without the extractors).
func TestWithoutSourcesSinksRustAndDart(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"rivus/rust", "rivus/dart"} {
		var had, left int
		var cut []*model.RuleSet
		for _, rs := range sets {
			if !strings.HasPrefix(rs.Name, lang) {
				continue
			}
			had += len(rs.Sources)
			c := rs.WithoutSourcesSinks()
			left += len(c.Sources) + len(c.Sinks)
			cut = append(cut, c)
		}
		if had == 0 {
			t.Errorf("%s: the built-in files have no sources", lang)
		}
		r := model.Merge(cut...)
		if left != 0 || len(r.CallSources)+len(r.ParamTypes)+len(r.Fields)+len(r.CallbackParams)+len(r.ParamFuncs)+len(r.Sinks) != 0 {
			t.Errorf("%s: sources or sinks are left", lang)
		}
	}
}
