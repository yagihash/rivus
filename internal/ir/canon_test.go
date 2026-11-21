package ir

import (
	"bytes"
	"testing"
)

func TestCanonTypeAndStream(t *testing.T) {
	p := &Program{TypeAliases: []TypeAlias{{Name: "a/b.Msg", Target: "a/internal.Msg"}, {Name: "a/b.MsgAlias2", Target: "a/b.Msg"},
		{Name: "a/b.PMsg", Target: "*a/b.Msg"}, {Name: "a/b.LMsg", Target: "[]*a/b.Msg"}, {Name: "a/b.MMsg", Target: "map[string]a/b.Msg"}}}
	c := p.CanonType()
	for in, want := range map[string]string{"a/b.Msg": "a/internal.Msg", "*a/b.Msg": "*a/internal.Msg", "[]*a/b.Msg": "[]*a/internal.Msg", "x.Y": "x.Y",
		// an alias inside a type: a map, a type argument, a function type, an alias of an alias
		"map[string]a/b.Msg":             "map[string]a/internal.Msg",
		"map[a/b.Msg][]*a/b.Msg":         "map[a/internal.Msg][]*a/internal.Msg",
		"p.Box[a/b.Msg]":                 "p.Box[a/internal.Msg]",
		"*p.Pair[a/b.Msg, a/b.Msg]":      "*p.Pair[a/internal.Msg, a/internal.Msg]",
		"func(a/b.Msg) (a/b.Msg, error)": "func(a/internal.Msg) (a/internal.Msg, error)",
		"a/b.MsgAlias2":                  "a/internal.Msg",
		// an alias of a pointer, a slice, a map, under the prefixes again (the type behind is read again)
		"*a/b.PMsg":           "**a/internal.Msg",
		"[]a/b.LMsg":          "[][]*a/internal.Msg",
		"map[string]a/b.MMsg": "map[string]map[string]a/internal.Msg",
		"a/b.MsgX":            "a/b.MsgX", // a name that only starts like an alias
	} {
		if got := c.Canon(in); got != want {
			t.Errorf("canon(%q) = %q, want %q", in, got, want)
		}
	}
	var buf bytes.Buffer
	if err := Write(&buf, p); err != nil {
		t.Fatal(err)
	}
	q, err := Read(&buf)
	if err != nil || len(q.TypeAliases) != 5 || q.TypeAliases[0] != p.TypeAliases[0] {
		t.Fatalf("round trip: %v %+v", err, q)
	}
}
