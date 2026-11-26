//go:build linux

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// The calls that bring the inode of a file outside into the work directory under a new name. The sandbox looks at the path of the file when it does a chmod for the
// program (notify_linux.go): if the inode could be given a name inside, a chmod of that name would change the file outside, and the check of the path would not see it.
// Landlock is what is to refuse these (a link or a rename from a directory that does not have the right REFER).
var (
	linkSrc = flag.String("link-src", "", "a directory outside with the files l1 to l5 in it (that may be read): they are given a name in link-dst by link, linkat, linkat of /proc/self/fd and rename, and the new name is chmod'ed")
	linkDst = flag.String("link-dst", "", "the directory inside (the work directory) where the new names are made")
)

func linkChecks() {
	if *linkSrc == "" || *linkDst == "" {
		return
	}
	src := func(n string) string { return filepath.Join(*linkSrc, n) }
	dst := func(n string) string { return filepath.Join(*linkDst, n) }
	// done says what came of the making of a name (ESCAPED when the name was made: then the file outside has a name inside) and of the chmod of the new name
	done := func(name, n string, err error) {
		how := fmt.Sprint(err)
		if err == nil {
			how += " chmod: " + fmt.Sprint(os.Chmod(dst(n), 0o640))
		}
		report(name, err == nil, how)
	}
	done("link-outside", "l1", os.Link(src("l1"), dst("l1")))
	done("linkat-outside", "l2", unix.Linkat(unix.AT_FDCWD, src("l2"), unix.AT_FDCWD, dst("l2"), 0))
	// the descriptor of the file, then its name in /proc (the way to a name for a file that was opened)
	f, err := os.Open(src("l3"))
	if err != nil {
		report("linkat-proc-fd-outside", false, "open: "+err.Error()) // the file could not be opened: nothing to link (the test reads it, so this is the test's mistake)
	} else {
		done("linkat-proc-fd-outside", "l3", unix.Linkat(unix.AT_FDCWD, fmt.Sprintf("/proc/self/fd/%d", f.Fd()), unix.AT_FDCWD, dst("l3"), unix.AT_SYMLINK_FOLLOW))
		f.Close()
	}
	done("rename-outside", "l4", os.Rename(src("l4"), dst("l4")))
	done("renameat2-outside", "l5", unix.Renameat2(unix.AT_FDCWD, src("l5"), unix.AT_FDCWD, dst("l5"), 0))
}
