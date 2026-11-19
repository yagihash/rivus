package report

import (
	"testing"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/sites"
)

func TestPackageOf(t *testing.T) {
	cases := map[string]string{
		"example.com/m/pkg.F":                     "example.com/m/pkg",
		"example.com/m/pkg.F$1":                   "example.com/m/pkg",
		"example.com/m/pkg.F$1$2":                 "example.com/m/pkg",
		"(*example.com/m/pkg.T).Method":           "example.com/m/pkg",
		"(example.com/m/pkg.T).Method":            "example.com/m/pkg",
		"gopkg.in/yaml.v3.Unmarshal":              "gopkg.in/yaml.v3",
		"example.com/m/pkg.Generic[go.shape.int]": "example.com/m/pkg",
		"main.main": "main",
		"":          "",
	}
	for in, want := range cases {
		if got := packageOf(in); got != want {
			t.Errorf("packageOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func site(id, structural, content string) *sites.Site {
	return &sites.Site{ID: id, Func: "p.F", Symbol: "p.Sink", Fingerprint: map[string]string{
		fingerprint.Structural: structural, fingerprint.Content: content,
		fingerprint.Expr: "e", fingerprint.Body: "b"}}
}

func TestResolveAmbiguousWhenTwoSitesShareTheContentKey(t *testing.T) {
	r := &Report{Packages: []string{"p"}, Sites: []*sites.Site{site("s1:a", "sa", "c"), site("s1:b", "sb", "c")}}
	res := r.Resolve([]*Bookmark{{ID: "s1:x", Func: "p.F", Fingerprint: map[string]string{fingerprint.Content: "c", fingerprint.Structural: "sa"}}})
	got := res.Results[0]
	if got.Status != StatusAmbiguous || len(got.Candidates) != 2 {
		t.Fatalf("want ambiguous with two candidates, got %+v", got)
	}
	if got.Current != nil {
		t.Error("an ambiguous answer must not pick one")
	}
}

func TestResolvePrefersContentOverStructure(t *testing.T) {
	// The old site has the structural key of site a and the content of site b.
	r := &Report{Packages: []string{"p"}, Sites: []*sites.Site{site("s1:a", "sa", "ca"), site("s1:b", "sb", "cb")}}
	res := r.Resolve([]*Bookmark{{ID: "s1:old", Func: "p.F", Fingerprint: map[string]string{fingerprint.Content: "cb", fingerprint.Structural: "sa", fingerprint.Expr: "e", fingerprint.Body: "b"}}})
	got := res.Results[0]
	if got.Current == nil || got.Current.ID != "s1:b" || got.MatchBasis != fingerprint.Content {
		t.Fatalf("content must win: %+v", got)
	}
	if got.Status != StatusRematched {
		t.Errorf("the ID differs, so the site was rematched: %s", got.Status)
	}
}

func TestResolveSiteWithoutKeysIsUnresolved(t *testing.T) {
	r := &Report{Packages: []string{"p"}, Sites: []*sites.Site{site("s1:a", "sa", "ca")}}
	res := r.Resolve([]*Bookmark{{ID: "s1:x", Func: "p.F", Fingerprint: map[string]string{"sim@1": "z"}}})
	if res.Results[0].Status != StatusUnresolved || res.Results[0].Reason != ReasonNotFound {
		t.Errorf("got %+v", res.Results[0])
	}
}

func TestUnitOf(t *testing.T) {
	for id, want := range map[string]string{
		"example.com/m/pkg.F":                                "example.com/m/pkg",
		"(*example.com/m/pkg.T).M":                           "example.com/m/pkg",
		"package:grpc_sample/hello.pb.dart#HelloRequest.new": "package:grpc_sample",
		"file:///work/app/bin/main.dart#main":                "file:",
		"anyhow::request_ref_backtrace":                      "anyhow",
		"<anyhow::_::MyError as std::fmt::Debug>::fmt":       "anyhow",
	} {
		if got := UnitOf(id); got != want {
			t.Errorf("UnitOf(%q) = %q, want %q", id, got, want)
		}
	}
}
