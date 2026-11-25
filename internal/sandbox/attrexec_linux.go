//go:build linux

package sandbox

import (
	"os"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// attrOutcome is what happened to a call that changes attributes.
type attrOutcome struct {
	errno   unix.Errno // what the program gets (0: it was done)
	refused string     // a file outside of the places that may be written: "name path"
	unread  bool       // the file or the arguments could not be read (the call was refused for that)
	gone    bool       // the program is gone: nothing is waiting for an answer
}

func toErrno(err error) unix.Errno {
	if err == nil {
		return 0
	}
	if e, ok := err.(unix.Errno); ok {
		return e
	}
	return unix.EIO
}

// tgidOf is the process (the thread group) that a thread belongs to: the number of the notification is a thread's, and pidfd_open wants the process.
func tgidOf(tid int) (int, bool) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(tid) + "/status")
	if err != nil {
		return 0, false
	}
	for _, l := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(l, "Tgid:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(rest))
			return n, err == nil
		}
	}
	return 0, false
}

// snapshot takes the open file that a descriptor of the program is now (pidfd_getfd): what it is at this moment is what is checked and what is changed, and a dup2
// of another thread afterwards does not change it. The errors that are the program's (the descriptor is not open: EBADF) are the answer; the ones that are rivus's
// (it may not take the descriptor: ptrace of the child is not allowed) are "unread", and the program being gone is "gone".
func snapshot(pid, n int) (f int, errno unix.Errno, unread, gone bool) {
	tgid, ok := tgidOf(pid)
	if !ok {
		return -1, 0, false, true
	}
	pfd, err := unix.PidfdOpen(tgid, 0)
	if err != nil {
		if toErrno(err) == unix.ESRCH {
			return -1, 0, false, true
		}
		return -1, unix.EPERM, true, false
	}
	defer unix.Close(pfd)
	f, err = unix.PidfdGetfd(pfd, n, 0)
	switch e := toErrno(err); e {
	case 0:
		return f, 0, false, false
	case unix.EBADF:
		return -1, unix.EBADF, false, false
	case unix.ESRCH:
		return -1, 0, false, true
	default:
		return -1, unix.EPERM, true, false
	}
}

// openProc opens a directory of the program in /proc (its working directory) to look from.
func openProc(pid int, name string) (f int, errno unix.Errno, unread bool) {
	f, err := unix.Open("/proc/"+strconv.Itoa(pid)+"/"+name, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, unix.EPERM, true
	}
	return f, 0, false
}

// fixProc says where the program means a path of /proc (and /dev/fd): /proc/self is the program and not rivus, which is where the path would be looked up.
func fixProc(path string, pid int) string {
	p := "/proc/" + strconv.Itoa(pid) + "/"
	for _, pre := range []string{"/proc/self/", "/proc/thread-self/"} {
		if rest, ok := strings.CutPrefix(path, pre); ok {
			return p + rest
		}
	}
	if rest, ok := strings.CutPrefix(path, "/dev/fd/"); ok {
		return p + "fd/" + rest
	}
	switch path {
	case "/dev/stdin":
		return p + "fd/0"
	case "/dev/stdout":
		return p + "fd/1"
	case "/dev/stderr":
		return p + "fd/2"
	}
	return path
}

// perform does the call for the program, if the file is inside of the places that may be written, and says what happened.
func (w *attrWatcher) perform(listener int, req *notifReq, call *attrCall) attrOutcome {
	pid := int(req.pid)
	arg := func(i int) uint64 { return req.data.args[i] }
	unread := attrOutcome{errno: unix.EPERM, unread: true}
	fail := func(e unix.Errno) attrOutcome { return attrOutcome{errno: e} }

	flags := 0
	if call.flags >= 0 {
		flags = int(int32(arg(call.flags)))
		if flags&^(atSymlinkNoFoll|atEmptyPath) != 0 {
			return fail(unix.EINVAL)
		}
	}
	nofollow := call.nofollow || flags&atSymlinkNoFoll != 0
	dirfd := atFDCWD
	if call.dirfd >= 0 {
		dirfd = int(int32(arg(call.dirfd)))
	}

	// 1. the arguments are copied from the memory of the program, once
	var path string
	pathNull := false
	if call.path >= 0 {
		if arg(call.path) == 0 {
			pathNull = true
		} else {
			p, ok := readString(pid, arg(call.path))
			if !ok {
				return unread
			}
			path = p
		}
	}
	var xname string
	if call.xname >= 0 {
		s, ok := readString(pid, arg(call.xname))
		if !ok {
			return unread
		}
		xname = s
	}
	var value []byte
	if call.size >= 0 {
		n := int(int64(arg(call.size)))
		if n < 0 || n > maxXattrValue {
			return fail(unix.E2BIG)
		}
		if n > 0 {
			v, ok := readBytes(pid, arg(call.value), n)
			if !ok {
				return unread
			}
			value = v
		}
	}
	var times []unix.Timespec
	if call.times >= 0 && arg(call.times) != 0 {
		b, ok := readBytes(pid, arg(call.times), int(unsafe.Sizeof([2]unix.Timespec{})))
		if !ok {
			return unread
		}
		times = make([]unix.Timespec, 2)
		copy((*[unsafe.Sizeof([2]unix.Timespec{})]byte)(unsafe.Pointer(&times[0]))[:], b)
	}

	// 2. the file: opened once, here, for the program (O_PATH), or the open file that its descriptor is now
	var f int
	switch {
	case call.fd >= 0:
		var errno unix.Errno
		var un, gone bool
		f, errno, un, gone = snapshot(pid, int(int32(arg(call.fd))))
		if gone {
			return attrOutcome{gone: true}
		}
		if un {
			return unread
		}
		if errno != 0 {
			return fail(errno)
		}
	case call.path >= 0 && ((pathNull && call.dirfd >= 0) || (!pathNull && path == "" && flags&atEmptyPath != 0)):
		// the descriptor itself (utimensat with no path, AT_EMPTY_PATH), or the working directory of the program
		var errno unix.Errno
		var un, gone bool
		if dirfd == atFDCWD {
			f, errno, un = openProc(pid, "cwd")
		} else {
			f, errno, un, gone = snapshot(pid, dirfd)
		}
		if gone {
			return attrOutcome{gone: true}
		}
		if un {
			return unread
		}
		if errno != 0 {
			return fail(errno)
		}
	case call.path >= 0:
		if pathNull {
			return fail(unix.EFAULT)
		}
		if path == "" {
			return fail(unix.ENOENT)
		}
		path = fixProc(path, pid)
		base := unix.AT_FDCWD
		if !strings.HasPrefix(path, "/") {
			var errno unix.Errno
			var un, gone bool
			if dirfd == atFDCWD {
				base, errno, un = openProc(pid, "cwd")
			} else {
				base, errno, un, gone = snapshot(pid, dirfd)
			}
			if gone {
				return attrOutcome{gone: true}
			}
			if un {
				return unread
			}
			if errno != 0 {
				return fail(errno)
			}
			defer unix.Close(base)
		}
		oflags := unix.O_PATH | unix.O_CLOEXEC
		if nofollow {
			oflags |= unix.O_NOFOLLOW
		}
		var err error
		f, err = unix.Openat(base, path, oflags, 0)
		if err != nil {
			return fail(toErrno(err)) // the errors of the program's own lookup: ENOENT, ENOTDIR, EACCES, ELOOP
		}
	default:
		return fail(unix.EINVAL)
	}
	defer unix.Close(f)

	// 3. the program is still there: it is its call that is answered
	if !idValid(listener, req.id) {
		return attrOutcome{gone: true}
	}

	// 4. the file that was opened is the one that is checked (the path of the open file, not a string that could be read again)
	self := "/proc/self/fd/" + strconv.Itoa(f)
	real, err := os.Readlink(self)
	if err != nil {
		return unread
	}
	if !strings.HasPrefix(real, "/") || strings.HasPrefix(real, "/memfd:") {
		return fail(unix.EPERM) // a pipe, a socket: not a file of the file system, and nothing of the program's files to change
	}
	real = strings.TrimSuffix(real, " (deleted)")
	if !w.isInside(real) {
		return attrOutcome{errno: unix.EPERM, refused: call.name + " " + real}
	}

	// 5. the change, on the file that was checked
	var st unix.Stat_t
	if err := unix.Fstatat(f, "", &st, unix.AT_EMPTY_PATH); err != nil {
		return fail(toErrno(err))
	}
	isLink := st.Mode&unix.S_IFMT == unix.S_IFLNK
	switch call.op {
	case opChmod:
		if isLink {
			return fail(unix.EOPNOTSUPP) // a link has no mode of its own
		}
		return fail(toErrno(unix.Fchmodat(unix.AT_FDCWD, self, uint32(arg(call.mode)), 0)))
	case opChown:
		return fail(toErrno(unix.Fchownat(f, "", int(int32(arg(call.uid))), int(int32(arg(call.gid))), unix.AT_EMPTY_PATH)))
	case opUtimes:
		return fail(toErrno(unix.UtimesNanoAt(f, "", times, unix.AT_EMPTY_PATH)))
	case opSetxattr:
		if isLink {
			return fail(unix.EPERM) // the attributes of the user are not for links
		}
		return fail(toErrno(unix.Setxattr(self, xname, value, int(int32(arg(call.xflags))))))
	case opRemovexattr:
		if isLink {
			return fail(unix.EPERM)
		}
		return fail(toErrno(unix.Removexattr(self, xname)))
	}
	return fail(unix.EINVAL)
}
