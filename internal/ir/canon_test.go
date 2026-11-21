package ir

import (
	"bytes"
	"testing"
)

func TestCanonTypeAndStream(t *testing.T) {
	p := &Program{TypeAliases: []TypeAlias{{Name: "a/b.Msg", Target: "a/internal.Msg"}}}
	c := p.CanonType()
	for in, want := range map[string]string{"a/b.Msg": "a/internal.Msg", "*a/b.Msg": "*a/internal.Msg", "[]*a/b.Msg": "[]*a/internal.Msg", "x.Y": "x.Y"} {
		if got := c(in); got != want {
			t.Errorf("canon(%q) = %q, want %q", in, got, want)
		}
	}
	var buf bytes.Buffer
	if err := Write(&buf, p); err != nil {
		t.Fatal(err)
	}
	q, err := Read(&buf)
	if err != nil || len(q.TypeAliases) != 1 || q.TypeAliases[0] != p.TypeAliases[0] {
		t.Fatalf("round trip: %v %+v", err, q)
	}
}
