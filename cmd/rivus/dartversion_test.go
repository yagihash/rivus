package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// VerifiedDart is the minor of DART_SDK of ci.yml: raising one without the other fails here (ADR-127).
func TestVerifiedDartMatchesCI(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s*DART_SDK: (\d+\.\d+)`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no DART_SDK in ci.yml")
	}
	if string(m[1]) != VerifiedDart {
		t.Errorf("ci.yml says DART_SDK %s, VerifiedDart is %q: change VerifiedDart in cmd/rivus/dartversion.go with it", m[1], VerifiedDart)
	}
}

func TestDartSDKConstraint(t *testing.T) {
	cases := []struct {
		name, pubspec, want string
		wantErr             bool
	}{
		{"a range", "name: a\nenvironment:\n  sdk: \">=3.0.0 <4.0.0\"\n", ">=3.0.0 <4.0.0", false},
		{"a caret", "name: a\nenvironment:\n  sdk: ^3.0.0\n", "^3.0.0", false},
		{"any", "name: a\nenvironment:\n  sdk: any\n", "any", false},
		{"a number", "name: a\nenvironment:\n  sdk: 3.0\n", "3.0", false},
		{"no sdk", "name: a\nenvironment:\n  flutter: \">=3.0.0\"\n", "", false},
		{"no environment", "name: a\n", "", false},
		{"broken", "name: a\nenvironment: [\n", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, filepath.Join(dir, "pubspec.yaml"), c.pubspec)
			got, err := dartSDKConstraint(dir)
			if (err != nil) != c.wantErr || got != c.want {
				t.Errorf("got %q, %v; want %q, error %v", got, err, c.want, c.wantErr)
			}
		})
	}
	if got, err := dartSDKConstraint(t.TempDir()); got != "" || err != nil {
		t.Errorf("no pubspec.yaml: %q, %v", got, err)
	}
}

func TestDartVersionOf(t *testing.T) {
	dir := t.TempDir()
	d, err := dartVersionOf(dir)
	if err != nil || d.Match != report.DartUnknown || d.Verified != VerifiedDart || d.Target != "" {
		t.Errorf("no pubspec.yaml: %+v, %v", d, err)
	}
	write(t, filepath.Join(dir, "pubspec.yaml"), "name: a\nenvironment:\n  sdk: \"^99.0.0\"\n")
	d, err = dartVersionOf(dir)
	if err != nil || d.Match != report.DartNewer || d.Target != "^99.0.0" || d.Minimum != "99.0.0" {
		t.Errorf("newer: %+v, %v", d, err)
	}
	write(t, filepath.Join(dir, "pubspec.yaml"), "name: a\nenvironment: [\n")
	d, err = dartVersionOf(dir)
	if err == nil || d.Match != report.DartUnknown {
		t.Errorf("broken: %+v, %v", d, err)
	}
}

func TestDartVersionWarning(t *testing.T) {
	if w := dartVersionWarning(report.CheckDartVersion("3.13", ">=3.0.0 <4.0.0")); w != "" {
		t.Errorf("a match gives no warning: %q", w)
	}
	if w := dartVersionWarning(report.CheckDartVersion("3.13", "")); w != "" || dartVersionWarning(nil) != "" {
		t.Errorf("unknown gives no warning: %q", w)
	}
	w := dartVersionWarning(report.CheckDartVersion("3.13", "^3.99.0"))
	for _, want := range []string{"rivus: WARNING: ", "sdk ^3.99.0", "Dart 3.99.0", "Dart 3.13", "(summary.dart_version)"} {
		if !strings.Contains(w, want) {
			t.Errorf("%q is not in %q", want, w)
		}
	}
}
