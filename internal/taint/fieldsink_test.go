package taint

import (
	"regexp"
	"strings"
	"testing"
)

// The sinks that name a part of their value (docs/DESIGN-field-sinks.md; testdata/go/cases/fieldsink).

func TestFieldSinksFindings(t *testing.T) { checkCase(t, "fieldsink") }

// TestFieldSinksSayWhenTheFieldIsNotTold holds the uncertainty of each finding: a flow that is told to be in the
// field has none; one that may be in it, but the value does not tell, has field_imprecision (with its
// why); a name the program does not have is field_unresolved. It holds the composition through the
// summaries too (a sink in a callee, two levels down), and that a sure way wins over an unsure one.
func TestFieldSinksSayWhenTheFieldIsNotTold(t *testing.T) {
	res, _ := runCase(t, "fieldsink")
	tag := regexp.MustCompile(`@sink\(([a-z]+)\)`)
	got := map[string][]string{} // tag -> the kinds (with the why) of the uncertainties of the field
	seen := map[string]bool{}
	for _, f := range res.Findings {
		m := tag.FindStringSubmatch(f.Sink.Snippet)
		if m == nil {
			continue
		}
		seen[m[1]] = true
		for _, u := range f.Uncertainties {
			switch u.Kind {
			case KindFieldImprecision:
				why := "?"
				if i := strings.Index(u.Detail, "why: "); i >= 0 {
					why = strings.SplitN(u.Detail[i+5:], ";", 2)[0]
				}
				got[m[1]] = append(got[m[1]], KindFieldImprecision+":"+why)
			case KindFieldUnresolved:
				got[m[1]] = append(got[m[1]], KindFieldUnresolved)
			}
		}
	}
	for _, tg := range []string{"direct", "literal", "viacall", "twolevels", "copy", "wrapped", "surewins", "surecall", "viaiface", "method"} {
		if !seen[tg] {
			t.Errorf("%s: no finding", tg)
		}
		if len(got[tg]) > 0 {
			t.Errorf("%s: a flow that is told to be in the field has uncertainty %v", tg, got[tg])
		}
	}
	want := map[string]string{
		"whole":        KindFieldImprecision + ":whole_value",
		"wholecall":    KindFieldImprecision + ":whole_value",
		"opaque":       KindFieldImprecision + ":opaque_callee",
		"opaqueinside": KindFieldImprecision + ":opaque_callee",
		"misspelled":   KindFieldUnresolved,
	}
	for tg, w := range want {
		if !seen[tg] {
			t.Errorf("%s: no finding", tg)
			continue
		}
		ok := false
		for _, g := range got[tg] {
			ok = ok || g == w
		}
		if !ok {
			t.Errorf("%s: uncertainties %v, want %s", tg, got[tg], w)
		}
	}
}

func TestMatchParts(t *testing.T) {
	cases := []struct {
		names []string
		rel   string
		kind  partKind
		rest  string
	}{
		{[]string{"SQL"}, "/SQL", partBelow, ""},
		{[]string{"SQL"}, "/*/SQL", partBelow, ""},        // the pointer in an interface
		{[]string{"SQL"}, "/SQL/*/x", partBelow, ""},      // below it
		{[]string{"SQL"}, "/Params/*/[]", partNone, ""},   // a field beside it
		{[]string{"SQL"}, "", partAbove, "SQL"},           // the value as a whole
		{[]string{"A", "B", "C"}, "/A", partAbove, "B.C"}, // a part that holds it
		{[]string{"A", "B", "C"}, "/A/*/B/*/C", partBelow, ""},
		{[]string{"A", "B", "C"}, "/A/*/X/*/C", partNone, ""}, // a name of the path is never left out
		{[]string{"[]", "SQL"}, "/[]/SQL", partBelow, ""},
		{[]string{"SQL"}, "/[]/SQL", partBelow, ""}, // an element that the rule did not write
		{[]string{"[]", "SQL"}, "/[]", partAbove, "SQL"},
		{[]string{"A", "B"}, "/A/*", partAbove, "B"},
	}
	for _, c := range cases {
		kind, rest := matchParts(c.names, c.rel)
		if kind != c.kind || strings.Join(rest, ".") != c.rest {
			t.Errorf("matchParts(%v, %q) = %v, %q; want %v, %q", c.names, c.rel, kind, strings.Join(rest, "."), c.kind, c.rest)
		}
	}
}
