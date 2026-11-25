package gossa

import (
	"os"
	"regexp"
	"testing"

	"golang.org/x/tools/go/packages"
)

// VerifiedGo is the minor of the go line of rivus's own go.mod: raising one without the other fails here (ADR-127).
func TestVerifiedGoMatchesOwnGoMod(t *testing.T) {
	b, err := os.ReadFile("../../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^go (\d+\.\d+)`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no go line in go.mod")
	}
	if string(m[1]) != VerifiedGo {
		t.Errorf("go.mod says go %s, VerifiedGo is %q: change VerifiedGo in internal/frontend/gossa/goversion.go with the go line", m[1], VerifiedGo)
	}
}

func TestGoVersionsOfModules(t *testing.T) {
	mod := func(path, goV string, main bool) *packages.Package {
		return &packages.Package{PkgPath: path + "/p", Module: &packages.Module{Path: path, GoVersion: goV, Main: main}}
	}
	std := &packages.Package{PkgPath: "fmt"} // no module
	repl := &packages.Package{PkgPath: "example.com/r/p", Module: &packages.Module{Path: "example.com/r", GoVersion: "1.30", Replace: &packages.Module{Path: "../r", GoVersion: "1.31"}}}
	main, got := goVersions([]*packages.Package{mod("example.com/app", "1.27", true), mod("example.com/dep", "1.99.1", false), mod("example.com/old", "", false), std, repl})
	if main != "example.com/app" {
		t.Errorf("main module = %q", main)
	}
	want := map[string]string{"example.com/app": "1.27", "example.com/dep": "1.99.1", "example.com/r": "1.31"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestLoadGoVersions(t *testing.T) {
	p, err := Load(testdata, []string{"./cases/hello"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.MainModule != "example.com/cases" || p.GoVersions["example.com/cases"] != "1.27" {
		t.Errorf("main module %q, go versions %v", p.MainModule, p.GoVersions)
	}
}
