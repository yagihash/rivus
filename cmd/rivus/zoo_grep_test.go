package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The two greps of docs/DESIGN-zoo.md section 11:
//   - no personal information (a home path, a mail address) in what the zoo holds (the repository is public);
//   - no special treatment of the zoo in the engine (internal/, and cmd/ apart from the tests): a name of the
//     zoo, of its sinks, or a marker pattern there would make the zoo green by what it is called.
func TestZooGreps(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	personal := regexp.MustCompile(`(/Users/|/home/[a-z]|[A-Za-z0-9._-]+@[A-Za-z0-9-]+\.[a-z]{2,})`)
	for _, d := range []string{"bench/zoo", "bench/zoo-go", "bench/zoo-dart", "bench/zoo-rust", "bench/zoo-go-pkg", "bench/zoo-rust-pkg"} {
		filepath.Walk(filepath.Join(root, d), func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if info.Name() == ".dart_tool" || info.Name() == "target" || info.Name() == "__pycache__" { // what pub, cargo and python write (absolute paths of this machine), not committed (.gitignore)
					return filepath.SkipDir
				}
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			if m := personal.Find(b); m != nil {
				rel, _ := filepath.Rel(root, p)
				t.Errorf("%s: %q looks like personal information", rel, m)
			}
			return nil
		})
	}
	engine := regexp.MustCompile(`(?i)(zoo/mark|mark\.Sink|\bzoo\b|Z[a-z0-9-]+\.SH)`)
	for _, d := range []string{"internal", "cmd"} {
		filepath.Walk(filepath.Join(root, d), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			if m := engine.Find(b); m != nil {
				rel, _ := filepath.Rel(root, p)
				t.Errorf("%s: %q: the engine must not know the zoo", rel, m)
			}
			return nil
		})
	}
}
