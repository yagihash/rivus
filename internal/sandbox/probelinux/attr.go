//go:build linux

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The checks of the calls that change attributes (the sandbox does them for the program, and only for the files that are inside of the places that may be written).
var (
	attrAll     = flag.String("attr-all", "", "a directory: every form of the calls is used on files that the probe makes in it, and what came of each one is printed (it is the same inside the sandbox and outside)")
	attrProcIn  = flag.String("attr-proc-in", "", "a file inside: chmod through /proc/self/fd/N of a descriptor of it")
	attrProcOut = flag.String("attr-proc-out", "", "a file that may be read and not written: chmod through /proc/self/fd/N of a descriptor of it")
	attrPipe    = flag.Bool("attr-pipe", false, "fchmod on a pipe")
	attrRoot    = flag.String("attr-root", "", "a file inside: chown to another user (only root can: the program has no capability, and neither may the one that does the call for it)")
	racePath    = flag.String("race-path", "", "a directory with A/ (inside) and B/ (outside) in it, with the same files f000...: fchmodat on a path that another thread changes from A to B")
	raceFd      = flag.String("race-fd", "", "inside-file:outside-file: fchmod on a descriptor that another thread changes (dup3) from the inside file to the outside one")
)

func errnoOf(err error) string {
	if err == nil {
		return "ok"
	}
	var e syscall.Errno
	if errors.As(err, &e) {
		return e.Error()
	}
	return err.Error()
}

// say prints what came of one use of a call.
func say(name string, err error) { fmt.Printf("%s: %s\n", name, errnoOf(err)) }

func attrExtra() {
	if *attrAll != "" {
		attrEverything(*attrAll)
	}
	if *attrProcIn != "" {
		f, err := os.Open(*attrProcIn)
		if err == nil {
			say("proc-in-chmod", os.Chmod(fmt.Sprintf("/proc/self/fd/%d", f.Fd()), 0o640))
			f.Close()
		} else {
			say("proc-in-open", err)
		}
	}
	if *attrProcOut != "" {
		f, err := os.Open(*attrProcOut)
		if err == nil {
			say("proc-out-chmod", os.Chmod(fmt.Sprintf("/proc/self/fd/%d", f.Fd()), 0o640))
			f.Close()
		} else {
			say("proc-out-open", err)
		}
	}
	if *attrPipe {
		r, w, err := os.Pipe()
		if err == nil {
			say("pipe-fchmod", unix.Fchmod(int(w.Fd()), 0o600))
			r.Close()
			w.Close()
		}
	}
	if *attrRoot != "" {
		say("chown-other", os.Chown(*attrRoot, 12345, 12345))
	}
	if *racePath != "" {
		raceOnPath(*racePath)
	}
	if *raceFd != "" {
		in, out, _ := strings.Cut(*raceFd, ":")
		raceOnFd(in, out)
	}
}

// attrEverything uses every form of the calls that the sandbox does for the program, and the ones that fail, and says what came of each.
func attrEverything(dir string) {
	f := filepath.Join(dir, "f")
	l := filepath.Join(dir, "l")
	os.WriteFile(f, []byte("data"), 0o644)
	os.Remove(l)
	os.Symlink("f", l)
	uid, gid := os.Getuid(), os.Getgid()
	fd, err := os.OpenFile(f, os.O_RDWR, 0)
	if err != nil {
		say("open", err)
		return
	}
	defer fd.Close()
	dirfd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		say("opendir", err)
		return
	}
	defer unix.Close(dirfd)
	t1 := unix.Timespec{Sec: 1000000001, Nsec: 111}
	t2 := unix.Timespec{Sec: 1000000002, Nsec: 222}
	say("chmod", os.Chmod(f, 0o640))
	say("chmod-missing", os.Chmod(filepath.Join(dir, "nothing"), 0o640))
	say("fchmod", fd.Chmod(0o600))
	say("fchmodat-dirfd", unix.Fchmodat(dirfd, "f", 0o644, 0))
	say("fchmodat-nofollow-on-link", unix.Fchmodat(dirfd, "l", 0o600, unix.AT_SYMLINK_NOFOLLOW))
	say("fchmodat-nofollow-on-file", unix.Fchmodat(dirfd, "f", 0o640, unix.AT_SYMLINK_NOFOLLOW))
	say("fchmodat-bad-flags", unix.Fchmodat(dirfd, "f", 0o640, 0x80000))
	say("chmod-through-link", os.Chmod(l, 0o600))
	say("chown", os.Chown(f, uid, gid))
	say("lchown-link", os.Lchown(l, uid, gid))
	say("fchown", fd.Chown(uid, gid))
	say("fchownat-nofollow", unix.Fchownat(dirfd, "l", uid, gid, unix.AT_SYMLINK_NOFOLLOW))
	say("fchownat-empty-path", unix.Fchownat(int(fd.Fd()), "", uid, gid, unix.AT_EMPTY_PATH))
	say("chown-unchanged", os.Chown(f, -1, -1))
	say("utimensat", unix.UtimesNanoAt(unix.AT_FDCWD, f, []unix.Timespec{t1, t2}, 0))
	say("utimensat-dirfd", unix.UtimesNanoAt(dirfd, "f", []unix.Timespec{t1, t2}, 0))
	ts := []unix.Timespec{t2, t1}
	_, _, e := unix.Syscall6(unix.SYS_UTIMENSAT, fd.Fd(), 0, uintptr(unsafe.Pointer(&ts[0])), 0, 0, 0) // futimens: utimensat with no path
	if e != 0 {
		say("futimens", e)
	} else {
		say("futimens", nil)
	}
	say("utimensat-omit", unix.UtimesNanoAt(unix.AT_FDCWD, f, []unix.Timespec{{Nsec: unix.UTIME_OMIT}, t2}, 0))
	say("utimensat-nofollow-link", unix.UtimesNanoAt(unix.AT_FDCWD, l, []unix.Timespec{t1, t1}, unix.AT_SYMLINK_NOFOLLOW))
	say("utimensat-empty-path", unix.UtimesNanoAt(int(fd.Fd()), "", []unix.Timespec{t1, t1}, unix.AT_EMPTY_PATH))
	say("setxattr", unix.Setxattr(f, "user.rivus", []byte("value"), 0))
	say("setxattr-create-again", unix.Setxattr(f, "user.rivus", []byte("value"), unix.XATTR_CREATE))
	say("setxattr-replace", unix.Setxattr(f, "user.rivus", []byte("other"), unix.XATTR_REPLACE))
	say("setxattr-link", unix.Lsetxattr(l, "user.rivus", []byte("x"), 0))
	say("fsetxattr", unix.Fsetxattr(int(fd.Fd()), "user.rivus2", []byte("v2"), 0))
	say("removexattr", unix.Removexattr(f, "user.rivus"))
	say("removexattr-missing", unix.Removexattr(f, "user.rivus"))
	say("fremovexattr", unix.Fremovexattr(int(fd.Fd()), "user.rivus2"))
	say("lremovexattr-link", unix.Lremovexattr(l, "user.rivus"))
	var st unix.Stat_t
	if err := unix.Stat(f, &st); err == nil {
		fmt.Printf("final: mode=%o mtime=%d.%d\n", st.Mode&0o7777, st.Mtim.Sec, st.Mtim.Nsec)
	}
}

// raceOnPath is the attack on the check of a path: one thread calls fchmodat on a buffer, and another changes one byte of the buffer (A to B) as fast as it can,
// between the moment that the call is looked at and the moment that the kernel would read the path again.
func raceOnPath(root string) {
	const n = 400
	buf := make([]byte, 512)
	pos := len(root) + 1
	stop := make(chan struct{})
	flip := 0
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			flip ^= 1
			buf[pos] = "AB"[flip]
		}
	}()
	for i := 0; i < n; i++ {
		p := fmt.Sprintf("%s/A/f%03d", root, i)
		copy(buf, p)
		buf[len(p)] = 0
		buf[pos] = 'A'
		unix.Syscall6(unix.SYS_FCHMODAT, uintptr(0xffffffffffffff9c), uintptr(unsafe.Pointer(&buf[0])), 0o640, 0, 0, 0)
	}
	close(stop)
}

// raceOnFd is the attack on the check of a descriptor: another thread makes the descriptor N refer to the inside file and to the outside one in turn.
func raceOnFd(inside, outside string) {
	in, err1 := os.OpenFile(inside, os.O_RDWR, 0)
	out, err2 := os.Open(outside)
	if err1 != nil || err2 != nil {
		fmt.Println("race-fd-open", errnoOf(err1), errnoOf(err2))
		return
	}
	n, err := unix.Dup(int(in.Fd()))
	if err != nil {
		fmt.Println("race-fd-dup", errnoOf(err))
		return
	}
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			unix.Dup3(int(out.Fd()), n, 0)
			unix.Dup3(int(in.Fd()), n, 0)
		}
	}()
	for i := 0; i < 2000; i++ {
		unix.Fchmod(n, 0o640)
	}
	close(stop)
}
