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

// The code under analysis is not trusted, and its .git/config is part of it: git runs what the
// configuration of a repository names (core.fsmonitor, and the filters that .gitattributes selects) when it
// asks for the status. Every call here switches those off first (yagihash/rivus #6).

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
	cmd := exec.Command("git", append(safeOptions(dir), args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// safeOptions are the options of git that keep the repository in dir from running a command of its own:
// no file system monitor, no optional locks, and every filter of its configuration (clean, smudge, process)
// emptied, so that a file whose stat differs is not passed through a program that the repository names.
// Reading the configuration runs nothing.
func safeOptions(dir string) []string {
	opts := []string{"-C", dir, "--no-optional-locks", "-c", "core.fsmonitor=false"}
	out, err := exec.Command("git", "-C", dir, "config", "--local", "--name-only", "--get-regexp", `^filter\.`).Output()
	if err != nil {
		return opts
	}
	seen := map[string]bool{}
	for _, key := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key = strings.TrimSpace(key)
		i, j := strings.Index(key, "."), strings.LastIndex(key, ".")
		if key == "" || i < 0 || j <= i {
			continue
		}
		name := key[i+1 : j]
		for _, what := range []string{"clean", "smudge", "process"} {
			k := "filter." + name + "." + what
			if !seen[k] {
				seen[k] = true
				opts = append(opts, "-c", k+"=")
			}
		}
	}
	return opts
}
