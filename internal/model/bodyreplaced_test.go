package model

import (
	"strings"
	"testing"
)

func TestReplacedBodyNamesThePackageAndNoOtherUnlessSaid(t *testing.T) {
	a, err := Parse(strings.NewReader("ruleset: a@1\nbody_replaced_packages:\n  - {package: example.com/m/a, reason: r}\n  - {package: example.com/m/t/..., reason: r}\n  - {package: \"package:googleapis\", reason: r}\n  - {package: serde, reason: r}\n"), "a.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := Merge(a)
	for unit, want := range map[string]bool{
		"example.com/m/a":         true,
		"example.com/m/a/b":       false, // the package named, and no other
		"example.com/m/ab":        false,
		"example.com/m/t":         true, // /...: the package and what is under it
		"example.com/m/t/x/y":     true,
		"example.com/m/tx":        false,
		"package:googleapis":      true,
		"package:googleapis_auth": false,
		"serde":                   true,
		"serde_json":              false,
		"file:":                   false,
		"":                        false,
	} {
		if got := r.ReplacedBody(unit); got != want {
			t.Errorf("ReplacedBody(%q) = %v, want %v", unit, got, want)
		}
	}
	if (*Rules)(nil).ReplacedBody("x") || Merge().ReplacedBody("x") {
		t.Error("no declaration declares nothing")
	}
	if got := r.WithDeclarations(nil); got.ReplacedBody("serde") || !r.ReplacedBody("serde") {
		t.Error("WithDeclarations(nil) leaves the declarations out, and does not change the rules it was made from")
	}
	// sorted: the order of the files, and of the entries, does not matter
	b, _ := Parse(strings.NewReader("ruleset: b@1\nbody_replaced_packages:\n  - {package: example.com/m/a, reason: r2}\n"), "b.yaml")
	d1, d2 := Merge(a, b).Declared, Merge(b, a).Declared
	if len(d1) != 5 || len(d2) != 5 {
		t.Fatalf("declarations: %d, %d", len(d1), len(d2))
	}
	for i := range d1 {
		if d1[i] != d2[i] {
			t.Errorf("declaration %d differs with the order of the sets: %+v, %+v", i, d1[i], d2[i])
		}
	}
}
