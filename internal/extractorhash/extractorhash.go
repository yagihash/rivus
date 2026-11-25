// Package extractorhash computes the hash of the source of an extractor (extractor/rust, extractor/dart): what it is made of, so that a
// change of its logic that leaves the shape of the IR alone (the schema version does not catch that) still changes the value. The extractor
// writes the hash in the first line of its IR, and rivus holds the hash of the source it was built with (-ldflags -X, see Makefile and
// scripts/build-release-archives.sh). scripts/extractor-hash prints it, and `rivus setup` passes it to the build of the extractor: one
// computation, one table of files (ADR-126).
package extractorhash

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// TableVersion goes at the head of what is hashed: a change of the table below (a file added or left out, the normalisation) changes every
// hash, so that two hashes of different tables are never equal by chance.
const TableVersion = "v1"

// Unknown is the value of an extractor or a rivus that was built without the hash (a plain `go build` or `cargo build`).
const Unknown = "unknown"

// A table lists, for a language, what the extractor is made of: the files of the directory itself, and the directories that are read as a
// whole (every file under them). Anything else in the directory is left out on purpose (Excluded), and the test of this package fails for a
// file or a directory that is in neither list, so that a new input of the build cannot stay out of the hash.
type table struct {
	Files []string
	Dirs  []string
}

var tables = map[string]table{
	// build.rs is in: it changes the build. Cargo.lock and rust-toolchain.toml are the dependencies and the compiler.
	"rust": {Files: []string{"build.rs", "Cargo.toml", "Cargo.lock", "rust-toolchain.toml"}, Dirs: []string{"src"}},
	// pubspec.lock is the version of the analyzer.
	"dart": {Files: []string{"pubspec.yaml", "pubspec.lock"}, Dirs: []string{"lib", "bin"}},
}

// Excluded are the names in the directory of an extractor that are not part of what it is made of: the tests, the products of a build, the
// documents.
var Excluded = map[string][]string{
	"rust": {"test", "target", "README.md", "LICENSE", ".dockerignore"},
	"dart": {"test", ".dart_tool", "README.md", "dart_test.yaml"},
}

// Langs are the languages that have an extractor, in order.
func Langs() []string { return []string{"dart", "rust"} }

// Listed says whether the name (a direct entry of the directory of the extractor) is in the table or in the exclusions.
func Listed(lang, name string) bool {
	t := tables[lang]
	for _, l := range [][]string{t.Files, t.Dirs, Excluded[lang]} {
		for _, n := range l {
			if n == name {
				return true
			}
		}
	}
	return false
}

// Compute is the hash (64 hexadecimal digits) of the extractor of lang whose source is in dir. For each file of the table, in the order of the
// slash-separated path (bytes), it takes the path, the length and the content, with CRLF read as LF (a checkout on Windows does not change the
// hash); the table version is first. A file of the table that is not there is an error.
func Compute(lang, dir string) (string, error) {
	t, ok := tables[lang]
	if !ok {
		return "", fmt.Errorf("extractorhash: no extractor for %q (rust or dart)", lang)
	}
	var files []string
	for _, f := range t.Files {
		files = append(files, f)
	}
	for _, d := range t.Dirs {
		err := filepath.WalkDir(filepath.Join(dir, d), func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !e.IsDir() {
				rel, err := filepath.Rel(dir, p)
				if err != nil {
					return err
				}
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(files)
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00", TableVersion)
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			return "", err
		}
		b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
