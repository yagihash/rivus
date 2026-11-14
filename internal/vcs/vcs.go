// Package vcs identifies the revision of the code that was analysed.
//
// The IDs of sites do not contain the revision, on purpose: they should
// stay the same when the code changes (ADR-028). The revision is recorded
// beside them, in every output, so that a reader knows which code a result
// describes, and a bookmark knows which code it was taken from.
package vcs

import (
	"os/exec"
	"strings"
)

// Target says which code an output describes.
type Target struct {
	// Revision is the commit the code is at (git HEAD).
	Revision string `json:"revision"`
	// Dirty is true when the working tree differs from the revision: the
	// analysed code is then not exactly that commit.
	Dirty bool `json:"dirty,omitempty"`
}

// String is the revision, with a "+dirty" suffix when the tree is modified.
func (t *Target) String() string {
	if t == nil {
		return ""
	}
	if t.Dirty {
		return t.Revision + "+dirty"
	}
	return t.Revision
}

// Detect returns the target of the code in dir, or nil when dir is not in a
// git working tree (or git is not available). The absence is not an error:
// the output just has no target.
func Detect(dir string) *Target {
	rev, err := git(dir, "rev-parse", "HEAD")
	if err != nil || rev == "" {
		return nil
	}
	st, err := git(dir, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return &Target{Revision: rev}
	}
	return &Target{Revision: rev, Dirty: st != ""}
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
