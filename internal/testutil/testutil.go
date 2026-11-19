// Package testutil holds helpers shared by the tests.
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// Testdata returns the absolute path of testdata/go.
func Testdata() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "go")
}

// Load loads packages of the test module.
func Load(t testing.TB, patterns ...string) *gossa.Program {
	t.Helper()
	p, err := gossa.Load(Testdata(), patterns, gossa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LoadErrors) != 0 {
		t.Fatalf("load errors: %v", p.LoadErrors)
	}
	return p
}

// Rules returns the test rules (testdata/go/rules.yaml) merged with the
// built-in rules.
func Rules(t testing.TB) *model.Rules {
	t.Helper()
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(Testdata(), "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rs, err := model.Parse(f, "rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rules := model.Merge(append(sets, rs)...)
	if err := rules.Validate(); err != nil {
		t.Fatal(err)
	}
	return rules
}
