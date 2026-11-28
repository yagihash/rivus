package docsgen

import (
	"strings"
	"testing"
)

func issue(t *testing.T, name, state string) Issue {
	t.Helper()
	is, err := Parse(name, "# 1: 題\n\n- 状態: "+state+"\n- 種類: 道具\n\n## 本文\n")
	if err != nil {
		t.Fatal(err)
	}
	return is
}

func TestParseState(t *testing.T) {
	for in, want := range map[string]string{
		"open":                    "open",
		"**wontfix**(消したので。2026)": "wontfix",
		"**closed**":              "closed",
		"":                        "",
	} {
		if got := issue(t, "1-a.md", in).State; got != want {
			t.Errorf("%q: state %q, want %q", in, got, want)
		}
	}
	if _, err := Parse("a.md", "# x\n"); err == nil {
		t.Error("a name without a number is an error")
	}
	if _, err := Parse("1-a.md", "# x\n\n- 状態: open\n- 状態: closed\n"); err == nil {
		t.Error("a key twice is an error")
	}
}

// A closed issue and one that will not be fixed are both finished: the table of the open ones is for the work that is left.
func TestTableSplitsOpenAndClosed(t *testing.T) {
	var is []Issue
	for i, st := range []string{"open", "partial", "deferred", "closed", "wontfix"} {
		x := issue(t, string(rune('1'+i))+"-a.md", st)
		x.Number = i + 1
		is = append(is, x)
	}
	out := Table(is)
	open, closed, ok := strings.Cut(out, "## 閉じたもの")
	if !ok {
		t.Fatal("no section of the closed ones")
	}
	for _, n := range []string{"#1]", "#2]", "#3]"} {
		if !strings.Contains(open, n) || strings.Contains(closed, n) {
			t.Errorf("%s must be in the open ones only", n)
		}
	}
	for _, n := range []string{"#4]", "#5]"} {
		if strings.Contains(open, n) || !strings.Contains(closed, n) {
			t.Errorf("%s must be in the closed ones only", n)
		}
	}
}
