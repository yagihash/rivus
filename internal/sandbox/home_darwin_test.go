//go:build darwin

package sandbox

import (
	"context"
	"testing"
)

// With no HOME the home directory cannot be named, and real("") is the current directory: the deny of the home directory would be for
// the wrong place. Run refuses before it makes the profile or starts anything (so this test runs nothing under the sandbox).
func TestRunRefusesWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	res, err := Run(context.Background(), Spec{Argv: []string{"/usr/bin/true"}})
	if err == nil {
		t.Fatalf("it ran with no home directory: %+v", res)
	}
}
