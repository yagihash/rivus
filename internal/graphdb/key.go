package graphdb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// The source hash is the key of a saved file: the file is used again only
// when what it was made from is exactly what there is now. It is made of
//
//   - the configuration: the flags, the rules, the version of rivus, the
//     environment and the toolchains (a list of names and values),
//   - the files the load read, each with the hash of its content, and
//   - the manifest: the hash of the list of the files of the tree, so that a
//     file that is new (that the load could now read) is noticed too.
//
// A doubt always means "make it again": a false "fresh" misses a change, and a
// false "stale" only costs the time of an index.

// KeyVersion is the version of how the hash is composed.
const KeyVersion = "rivus.srckey/1"

// Entry is a file to hash.
type Entry struct {
	// Path is the name the file is stored under, Abs where it is.
	Path, Abs, Kind string
}

// HashFiles hashes the content of the entries, in parallel. The result is in
// the order of the entries. A file that cannot be read is recorded with the
// hash "unreadable": it is the same at the next check, and a file that becomes
// readable is a change. since, if not zero, is when the load started: a file
// that was written after that may have changed while it was read, and is an
// error (the index would claim a state that no one saw).
func HashFiles(entries []Entry, since time.Time) ([]FileRec, error) {
	out := make([]FileRec, len(entries))
	errs := make([]error, len(entries))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 2*runtime.NumCPU())
	for i := range entries {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			e := entries[i]
			rec := FileRec{Path: e.Path, Kind: e.Kind}
			sum, size, mod, err := hashFile(e.Abs)
			if err != nil {
				rec.SHA256, rec.Size = "unreadable", -1
			} else {
				rec.SHA256, rec.Size = sum, size
				if !since.IsZero() && !mod.Before(since) && guarded(e.Kind) {
					errs[i] = fmt.Errorf("%s changed while it was being read: run it again once the files are at rest", e.Path)
				}
			}
			out[i] = rec
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// guarded: the files whose change during the load is an error. The lock files
// are not: a tool the load runs may write one (cargo makes a Cargo.lock).
func guarded(kind string) bool { return kind != "lock" && kind != "module" && kind != "generated" }

func hashFile(path string) (string, int64, time.Time, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return "", 0, time.Time{}, err
	}
	h := sha256.New()
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		// a link is its target's name: what it leads to is read through its own path if the load did
		t, err := os.Readlink(path)
		if err != nil {
			return "", 0, time.Time{}, err
		}
		io.WriteString(h, "link:"+t)
		return hex.EncodeToString(h.Sum(nil)), int64(len(t)), fi.ModTime(), nil
	case !fi.Mode().IsRegular():
		return "", 0, time.Time{}, fmt.Errorf("%s: not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, time.Time{}, err
	}
	defer f.Close()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, time.Time{}, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, fi.ModTime(), nil
}

// HashBytes is the SHA-256 of s, in hex.
func HashBytes(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// SkipDir says which directories the listing of a tree leaves out: the
// version control data, the places tools put what they make, the output of the
// build. A directory that the analysis can read (a hidden one that holds
// source, as lib/.generated does) is not left out.
func SkipDir(root, dir string, name string) bool {
	switch name {
	case ".git", ".hg", ".svn", ".dart_tool", ".rivus", "node_modules", ".gradle", ".idea", ".vscode":
		return true
	case "build", "target":
		// build output at the top of a project; deeper, a package may have such a name
		return filepath.Dir(dir) == root
	}
	return false
}

// ListTree lists the files of the directories in roots (the first is the
// project: its files are named relative to it, the others by their absolute
// path), without the directories SkipDir leaves out, and without the paths in
// exclude (the file of the graph itself). A link is listed by its name and not
// followed. The list is sorted.
func ListTree(roots []string, exclude map[string]bool) ([]Entry, error) {
	var out []Entry
	type root struct {
		dir   string
		first bool
	}
	var kept []root
	for i, r := range roots {
		inside := false
		for _, k := range kept {
			if r == k.dir || strings.HasPrefix(r, k.dir+string(filepath.Separator)) {
				inside = true
			}
		}
		if !inside {
			kept = append(kept, root{r, i == 0})
		}
	}
	for _, rt := range kept {
		display := func(p string) string {
			if rt.first {
				if r, err := filepath.Rel(rt.dir, p); err == nil {
					return filepath.ToSlash(r)
				}
			}
			return filepath.ToSlash(p)
		}
		err := filepath.WalkDir(rt.dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if p == rt.dir {
					return err
				}
				// a directory that cannot be listed is a doubt, not a pass
				out = append(out, Entry{Path: display(p) + "/?unreadable", Abs: p, Kind: "tree"})
				return nil
			}
			if d.IsDir() {
				if p != rt.dir && SkipDir(rt.dir, p, d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if exclude[p] {
				return nil
			}
			out = append(out, Entry{Path: display(p), Abs: p, Kind: "tree"})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Manifest is the hash of a list of names.
func Manifest(entries []Entry) string {
	h := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(h, "%d:%s\n", len(e.Path), e.Path)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Compose makes the source hash from its parts.
func Compose(config map[string]string, files []FileRec, manifest string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", KeyVersion)
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "cfg %d:%s %d:%s\n", len(k), k, len(config[k]), config[k])
	}
	fs := append([]FileRec(nil), files...)
	sort.Slice(fs, func(i, j int) bool { return fs[i].Path < fs[j].Path })
	for _, f := range fs {
		fmt.Fprintf(h, "file %d:%s %s %s\n", len(f.Path), f.Path, f.Kind, f.SHA256)
	}
	fmt.Fprintf(h, "manifest %s\n", manifest)
	return hex.EncodeToString(h.Sum(nil))
}

// Parts is the key as made now, to be held against the one a file recorded.
type Parts struct {
	Config   map[string]string
	Files    []FileRec
	Manifest string
}

// Differences says what is not the same in the two keys, for a reader: one
// line for each thing, the first max of them (and a count of the rest).
func Differences(stored *KeyParts, now *Parts, max int) []string {
	var out []string
	add := func(s string) { out = append(out, s) }
	keys := map[string]bool{}
	for k := range stored.Config {
		keys[k] = true
	}
	for k := range now.Config {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		a, aok := stored.Config[k]
		b, bok := now.Config[k]
		switch {
		case aok && !bok:
			add(fmt.Sprintf("%s: was set, now it is not", k))
		case !aok && bok:
			add(fmt.Sprintf("%s: was not set, now it is", k))
		case a != b:
			add(fmt.Sprintf("%s: changed (%s -> %s)", k, shorten(a), shorten(b)))
		}
	}
	old := map[string]FileRec{}
	for _, f := range stored.Files {
		old[f.Path] = f
	}
	seen := map[string]bool{}
	for _, f := range now.Files {
		seen[f.Path] = true
		o, ok := old[f.Path]
		switch {
		case !ok:
			add(fmt.Sprintf("file added: %s", f.Path))
		case o.SHA256 != f.SHA256:
			add(fmt.Sprintf("file changed: %s", f.Path))
		}
	}
	for _, f := range stored.Files {
		if !seen[f.Path] {
			add(fmt.Sprintf("file gone: %s", f.Path))
		}
	}
	if stored.Manifest != now.Manifest {
		add("the list of the files of the tree changed (a file was added, removed or renamed)")
	}
	if len(out) > max {
		rest := len(out) - max
		out = append(out[:max], fmt.Sprintf("... and %d more", rest))
	}
	return out
}

func shorten(s string) string {
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}
