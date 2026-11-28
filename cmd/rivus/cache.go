package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// rivus cache list / rivus cache clean: the places of the dependencies of Rust (rivus-cargo-<hash>, directly under rustDataDir()) grow with every change of
// Cargo.lock and nothing removes them (issue 199). The commands look at those places and nothing else: a name that is not rivus-cargo- and 16 hex digits
// (extractor/, anything else the user keeps there), a symbolic link, and a place whose real path is outside rustDataDir() are never listed and never removed.

// cargoPlaceName is the name of a place of the dependencies (cargoVolume makes it).
var cargoPlaceName = regexp.MustCompile(`^rivus-cargo-[0-9a-f]{16}$`)

// inUseWithin: a place that was written within this time is taken as in use and is not removed. rivus holds no lock on a place while it fetches or
// extracts, so the time of the last write is the only sign there is.
const inUseWithin = time.Hour

type cachePlace struct {
	Name string
	Path string
	Size int64
	Last time.Time // the newest modification time of the place and of what is in it
}

// cachePlaces lists the places under root. A missing root is no place and no error. Entries that do not pass the checks are left out.
func cachePlaces(root string) ([]cachePlace, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []cachePlace
	for _, e := range entries {
		p := filepath.Join(root, e.Name())
		if !safeCachePlace(realRoot, e.Name(), p) {
			continue
		}
		size, last, err := measure(p)
		if err != nil {
			return nil, err
		}
		out = append(out, cachePlace{Name: e.Name(), Path: p, Size: size, Last: last})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// safeCachePlace says whether p (named name, directly under the root whose real path is realRoot) is a place that may be listed and removed: the name is of
// the form, it is a directory and not a link, and its real path is realRoot/name.
func safeCachePlace(realRoot, name, p string) bool {
	if !cargoPlaceName.MatchString(name) || strings.Contains(name, "..") || strings.ContainsRune(name, filepath.Separator) {
		return false
	}
	fi, err := os.Lstat(p)
	if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return false
	}
	real, err := filepath.EvalSymlinks(p)
	return err == nil && real == filepath.Join(realRoot, name)
}

// measure is the sum of the sizes of the files under p (links are not followed) and the newest modification time.
func measure(p string) (int64, time.Time, error) {
	var size int64
	var last time.Time
	err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		if fi.ModTime().After(last) {
			last = fi.ModTime()
		}
		if fi.Mode().IsRegular() {
			size += fi.Size()
		}
		return nil
	})
	return size, last, err
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}

func printCachePlaces(w io.Writer, root string, ps []cachePlace) {
	if len(ps) == 0 {
		fmt.Fprintf(w, "no place of the dependencies of Rust (rivus-cargo-<hash>) in %s\n", root)
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PATH\tSIZE\tBYTES\tLAST WRITTEN")
	for _, p := range ps {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", p.Path, humanSize(p.Size), p.Size, p.Last.Format("2006-01-02"))
	}
	tw.Flush()
}

func cacheList(root string, w io.Writer) error {
	ps, err := cachePlaces(root)
	if err != nil {
		return err
	}
	printCachePlaces(w, root, ps)
	return nil
}

// maxOlderThanDays is the most that -older-than takes (100 years).
const maxOlderThanDays = 36500

// cacheClean lists the places that were last written (the newest modification time inside) more than days days before now. It removes them only when yes is set (no question), or when ask is
// not nil and the user answers yes; with neither it only lists. A place that was written within inUseWithin is left.
func cacheClean(root string, days int, now time.Time, yes bool, ask func(prompt string) bool, w io.Writer) error {
	// time.Duration is 64 bits of nanoseconds (about 106000 days): more would overflow to a cut in the future, and every place would be "old"
	if days < 0 || days > maxOlderThanDays {
		return fmt.Errorf("-older-than is a number of days, 0 to %d (got %d)", maxOlderThanDays, days)
	}
	ps, err := cachePlaces(root)
	if err != nil {
		return err
	}
	cut := now.Add(-time.Duration(days) * 24 * time.Hour)
	var old []cachePlace
	for _, p := range ps {
		switch {
		case !p.Last.Before(cut):
		case now.Sub(p.Last) < inUseWithin:
			fmt.Fprintf(w, "skipped (written within the last hour, taken as in use): %s\n", p.Path)
		default:
			old = append(old, p)
		}
	}
	if len(old) == 0 {
		fmt.Fprintf(w, "nothing last written more than %d days ago in %s\n", days, root)
		return nil
	}
	printCachePlaces(w, root, old)
	if !yes {
		if ask == nil {
			fmt.Fprintf(w, "nothing was removed: these are the places that -yes would remove (last written more than %d days ago)\n", days)
			return nil
		}
		if !ask(fmt.Sprintf("remove these %d places? [y/N] ", len(old))) {
			fmt.Fprintln(w, "nothing was removed")
			return nil
		}
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	for _, p := range old {
		// check again just before: the place may have changed since the list
		if !safeCachePlace(realRoot, p.Name, p.Path) {
			fmt.Fprintf(w, "skipped (not a place of rivus any more): %s\n", p.Path)
			continue
		}
		if _, last, err := measure(p.Path); err != nil || now.Sub(last) < inUseWithin {
			fmt.Fprintf(w, "skipped (written just now, taken as in use): %s\n", p.Path)
			continue
		}
		if err := removePlace(p.Path); err != nil {
			return err
		}
		fmt.Fprintf(w, "removed %s (%s)\n", p.Path, humanSize(p.Size))
	}
	return nil
}

// removePlace removes the directory p. cargo makes files without the write right, so the right is given to what is under p first (links are not followed).
func removePlace(p string) error {
	_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				_ = os.Chmod(path, fi.Mode().Perm()|0o700)
			}
		}
		return nil
	})
	return os.RemoveAll(p)
}

type cacheCleanCmd struct {
	olderThan *int
	yes       *bool
}

func cacheCleanFlags(fs *flag.FlagSet) *cacheCleanCmd {
	c := &cacheCleanCmd{}
	c.olderThan = fs.Int("older-than", -1, "remove the places that were last written more than this many days ago (required, 0 to 36500). The date is the newest modification time inside a place: a run that only reads it (the same Cargo.lock fetches nothing new) does not move it, so a place that you use every day can be old here, and rivus cannot tell that one is in use except by a write within the last hour. Without -yes the places are listed, and on a terminal you are asked before they are removed")
	c.yes = fs.Bool("yes", false, "remove without asking")
	usual(fs, "older-than", "yes")
	return c
}

func runCacheList(argv []string) error {
	fs := flag.NewFlagSet("cache list", flag.ContinueOnError)
	prepareFlags(fs, argv)
	if err := parseFlags(fs, argv); err != nil {
		return err
	}
	root, err := rustDataDir()
	if err != nil {
		return err
	}
	return cacheList(root, os.Stdout)
}

func runCacheClean(argv []string) error {
	fs := flag.NewFlagSet("cache clean", flag.ContinueOnError)
	c := cacheCleanFlags(fs)
	prepareFlags(fs, argv)
	if err := parseFlags(fs, argv); err != nil {
		return err
	}
	given := false
	fs.Visit(func(f *flag.Flag) { given = given || f.Name == "older-than" })
	if !given {
		return fmt.Errorf("-older-than <days> is required (rivus cache clean -older-than 30 lists the places that would go)")
	}
	root, err := rustDataDir()
	if err != nil {
		return err
	}
	var ask func(string) bool
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		ask = confirm // a terminal: ask. Otherwise (a pipe, a script) only list; -yes is the way to remove
	}
	return cacheClean(root, *c.olderThan, time.Now(), *c.yes, ask, os.Stdout)
}
