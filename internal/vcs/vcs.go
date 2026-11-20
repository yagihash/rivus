// Package vcs identifies the revision of the code that was analysed.
//
// The IDs of sites do not contain the revision, on purpose: they should
// stay the same when the code changes (ADR-028). The revision is recorded
// beside them, in every output, so that a reader knows which code a result
// describes, and a bookmark knows which code it was taken from.
//
// The code under analysis is not trusted, and its .git/config is part of it: git runs what the
// configuration of a repository names (core.fsmonitor, filters, includes, promisor remotes) when it is asked
// for the status. So git is not run at all: the revision is read from the files of the repository, and whether
// the working tree differs from it is not looked at (yagihash/rivus #6, ADR-038 addendum).
package vcs

import (
	"os"
	"path/filepath"
	"strings"
)

// Target says which code an output describes.
type Target struct {
	// Revision is the commit the code is at (git HEAD). The working tree may differ from it.
	Revision string `json:"revision"`
}

// String is the revision.
func (t *Target) String() string {
	if t == nil {
		return ""
	}
	return t.Revision
}

// Detect returns the target of the code in dir, or nil when dir is not in a
// git working tree. The absence is not an error: the output just has no target.
func Detect(dir string) *Target {
	gitDir, ok := findGitDir(dir)
	if !ok {
		return nil
	}
	rev := readHead(gitDir)
	if rev == "" {
		return nil
	}
	return &Target{Revision: rev}
}

// findGitDir walks up from dir to the first .git, a directory or a file ("gitdir: path", as a worktree has).
func findGitDir(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		p := filepath.Join(dir, ".git")
		if fi, err := os.Stat(p); err == nil {
			if fi.IsDir() {
				return p, true
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return "", false
			}
			s := strings.TrimSpace(string(b))
			if !strings.HasPrefix(s, "gitdir:") {
				return "", false
			}
			g := strings.TrimSpace(strings.TrimPrefix(s, "gitdir:"))
			if !filepath.IsAbs(g) {
				g = filepath.Join(dir, g)
			}
			return filepath.Clean(g), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// readHead is the commit that HEAD names, or "" when it cannot be told (a format that is not read here,
// such as reftable, is one of those).
func readHead(gitDir string) string {
	b, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(b))
	if !strings.HasPrefix(head, "ref:") {
		if isHash(head) {
			return head
		}
		return ""
	}
	ref := strings.TrimSpace(strings.TrimPrefix(head, "ref:"))
	if !strings.HasPrefix(ref, "refs/") || strings.Contains(ref, "..") {
		return ""
	}
	// A linked worktree keeps HEAD of its own, and the refs of the main repository.
	dirs := []string{gitDir}
	if c, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		d := strings.TrimSpace(string(c))
		if !filepath.IsAbs(d) {
			d = filepath.Join(gitDir, d)
		}
		dirs = append(dirs, filepath.Clean(d))
	}
	for _, d := range dirs {
		if b, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(ref))); err == nil {
			if h := strings.TrimSpace(string(b)); isHash(h) {
				return h
			}
		}
	}
	for _, d := range dirs {
		b, err := os.ReadFile(filepath.Join(d, "packed-refs"))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) == 2 && f[1] == ref && isHash(f[0]) {
				return f[0]
			}
		}
	}
	return ""
}

func isHash(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
