package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDocLinks: every relative link `[text](path.md)` in README.md and docs/**/*.md points to a file that exists (a move
// or a rename of a document must change the links). A link with a `#anchor` is checked for the file only. Links in code
// blocks and inline code are not links.
func TestDocLinks(t *testing.T) {
	root := filepath.Join("..", "..")
	var files []string
	files = append(files, filepath.Join(root, "README.md"))
	err := filepath.WalkDir(filepath.Join(root, "docs"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// links that are broken today and are not fixed here (path of the file from the root -> the targets); empty: none
	allow := map[string][]string{}
	linkRe := regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+\.md)(#[^)]*)?\)`)
	inline := regexp.MustCompile("`[^`]*`")
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue // a generated file that is not there (docs/ISSUES-TABLE.md is ignored by git)
		}
		rel, _ := filepath.Rel(root, f)
		fence := false
		for n, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "```") {
				fence = !fence
				continue
			}
			if fence {
				continue
			}
			for _, m := range linkRe.FindAllStringSubmatch(inline.ReplaceAllString(l, ""), -1) {
				target := m[1]
				if strings.Contains(target, "://") || strings.HasPrefix(target, "/") {
					continue
				}
				if _, err := os.Stat(filepath.Join(filepath.Dir(f), target)); err != nil {
					ok := false
					for _, a := range allow[filepath.ToSlash(rel)] {
						ok = ok || a == target
					}
					if !ok {
						t.Errorf("%s:%d: the link %q points to no file", filepath.ToSlash(rel), n+1, target)
					}
				}
			}
		}
	}
}
