package ir

import (
	"strings"
	"testing"
)

func TestParentFuncIDs(t *testing.T) {
	for _, c := range []struct {
		id   string
		want []string
	}{
		{"p#f", []string{"p#f"}},
		{"p#f$1", []string{"p#f$1", "p#f"}},
		{"p#f$1$2", []string{"p#f$1$2", "p#f$1", "p#f"}},
		{"p#_$FromJson", []string{"p#_$FromJson"}},
		{"p#_$FromJson$3", []string{"p#_$FromJson$3", "p#_$FromJson"}},
		{"p#a$b", []string{"p#a$b"}},
		{"p#C.$copy", []string{"p#C.$copy"}},
		{"p#f$", []string{"p#f$"}},
	} {
		got := ParentFuncIDs(c.id)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%s: %v, want %v", c.id, got, c.want)
		}
	}
}
