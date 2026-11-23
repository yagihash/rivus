package main

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// A type error in one line of a package takes the packages that import it out of the analysis (go/ssa
// builds nothing for them), and they stay in `packages`. The output says which packages were read and
// not built and how many functions that is, in summary.unread and as a missing_source uncertainty for
// each; the findings and the exit status do not change (testdata/go/cases/illtyped).
func TestUnreadPackagesAreTold(t *testing.T) {
	t.Parallel()
	var rep report.Report
	rivusJSON(t, &rep, "analyze", "-C", "cases/illtyped", "-rules", "cases/illtyped/rules.yaml", "./...")

	u := rep.Summary.Unread
	if u == nil {
		t.Fatalf("summary.unread is absent: %+v", rep.Summary)
	}
	// broken (2 functions: the type error is in it), user (2), top (1); fine is built
	if u.Packages != 3 || u.Functions != 5 {
		t.Errorf("unread = %d packages, %d functions; want 3 and 5", u.Packages, u.Functions)
	}
	var names []string
	for _, p := range u.Top {
		names = append(names, strings.TrimPrefix(p.Package, "example.com/illtyped/"))
		if !strings.Contains(p.Why, "does not type-check") {
			t.Errorf("%s: the reason is missing: %q", p.Package, p.Why)
		}
	}
	// the most functions first, then the name
	if got := strings.Join(names, " "); got != "broken user top" {
		t.Errorf("top = %q, want %q", got, "broken user top")
	}
	// a package that depends on the one with the error says whose error it is
	for _, p := range u.Top {
		if strings.HasSuffix(p.Package, "/user") && !strings.Contains(p.Why, "/broken") {
			t.Errorf("user does not name the package it depends on: %q", p.Why)
		}
	}

	// the packages stay listed, and the flow of the package that was built is still found; nothing
	// of the unread ones is (what the run does not see is not a finding)
	if len(rep.Packages) != 5 {
		t.Errorf("packages = %v, want the five of the case (lib, broken, user, top, fine)", rep.Packages)
	}
	if len(rep.Findings) != 1 || !strings.HasSuffix(rep.Findings[0].Sink.Position.File, "fine/fine.go") {
		t.Errorf("findings = %d, want the one of fine", len(rep.Findings))
	}

	// each unread package is a missing_source uncertainty
	told := map[string]bool{}
	for _, un := range rep.Uncertainties {
		if un.Kind == "missing_source" && strings.HasPrefix(un.Detail, "not analysed:") {
			told[strings.TrimPrefix(un.Symbol, "example.com/illtyped/")] = true
		}
	}
	for _, n := range []string{"broken", "user", "top"} {
		if !told[n] {
			t.Errorf("no missing_source uncertainty for %s: %v", n, told)
		}
	}
	if told["fine"] {
		t.Error("fine was built, and is told as unread")
	}
}

// A run in which every package builds has no summary.unread.
func TestUnreadIsAbsentWhenEverythingBuilt(t *testing.T) {
	t.Parallel()
	var rep report.Report
	rivusJSON(t, &rep, "analyze", "-rules", "rules.yaml", "./cases/hello")
	if rep.Summary.Unread != nil {
		t.Errorf("summary.unread = %+v for a program that built", rep.Summary.Unread)
	}
}
