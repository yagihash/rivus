//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The calls that change the attributes of a file (attrCalls) are not stopped by Landlock: a program can change the mode of a file of the same user
// that it may not read or write. They are not refused here either (a linker changes the mode of its output with chmod; a path cannot be looked at by a
// filter of seccomp), but they are handed to rivus, which looks at the path and reports the ones that are outside of the places that the run may write.
// The report is after the fact and it is not a guard: what is told is what the program asked for.

// attrCall says where the arguments of a system call that changes attributes are (-1: there is none of that kind).
type attrCall struct {
	nr       uint32
	name     string
	fd       int  // the file descriptor of the file
	dirfd    int  // the directory that path is relative to
	path     int  // the path (a string)
	flags    int  // the flags of the call (AT_SYMLINK_NOFOLLOW, AT_EMPTY_PATH)
	nofollow bool // the call is about the link itself, not about what it points to (lchown, lsetxattr, lremovexattr)
}

// the forms of the calls: by a path (argument 0), by a descriptor (argument 0), and relative to a directory (the descriptor of the directory is argument 0 and the
// path argument 1; flags is the number of the argument that holds AT_* flags, -1 when there is none)
func byPath(nr uint32, name string, nofollow bool) attrCall {
	return attrCall{nr: nr, name: name, fd: -1, dirfd: -1, path: 0, flags: -1, nofollow: nofollow}
}

func byFd(nr uint32, name string) attrCall {
	return attrCall{nr: nr, name: name, fd: 0, dirfd: -1, path: -1, flags: -1}
}

func byDir(nr uint32, name string, flags int) attrCall {
	return attrCall{nr: nr, name: name, fd: -1, dirfd: 0, path: 1, flags: flags}
}

const (
	atFDCWD         = -100
	atSymlinkNoFoll = 0x100
	atEmptyPath     = 0x1000
	notifIDValid    = 0x40082102 // SECCOMP_IOCTL_NOTIF_ID_VALID (the same on x86-64 and arm64)
	maxChangesKept  = 200
	maxPathLen      = 4096
	notifPollMillis = 100
)

// seccomp_notif and seccomp_notif_resp (linux/seccomp.h)
type notifReq struct {
	id    uint64
	pid   uint32
	flags uint32
	data  struct {
		nr   int32
		arch uint32
		ip   uint64
		args [6]uint64
	}
}

type notifResp struct {
	id    uint64
	val   int64
	errno int32
	flags uint32
}

// attrWatcher reads the notifications of a run and keeps what it saw outside of the places that the run may write.
type attrWatcher struct {
	inside []string // real paths of the directories that the run may write
	mu     sync.Mutex
	seen   map[string]bool
	list   []string
	cut    int
	unread int    // the calls whose path could not be read
	lost   string // the reading of the notifications was given up
	recv   func(fd int, req *notifReq) unix.Errno
}

func newAttrWatcher(write []string) *attrWatcher {
	return &attrWatcher{inside: write, seen: map[string]bool{}}
}

// receiveFd reads the descriptor of the notifications that the helper sends over the socket (it sends nothing and closes it when there is none). The
// descriptor is close-on-exec from the moment that it is received (MSG_CMSG_CLOEXEC): rivus starts other processes while a run goes on, and none of them may
// get it (it would keep the listener open after the serving, and could answer the notifications).
func receiveFd(sock int) (int, bool) {
	buf := make([]byte, 1)
	oob := make([]byte, unix.CmsgSpace(4))
	n, oobn, _, _, err := unix.Recvmsg(sock, buf, oob, unix.MSG_CMSG_CLOEXEC)
	if err != nil || (n == 0 && oobn == 0) {
		return -1, false
	}
	msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil || len(msgs) == 0 {
		return -1, false
	}
	fds, err := unix.ParseUnixRights(&msgs[0])
	if err != nil || len(fds) == 0 {
		return -1, false
	}
	unix.CloseOnExec(fds[0])
	return fds[0], true
}

// receive takes the descriptor of the notifications and serves it until done is closed. The descriptor is closed only after that: see serve.
func (w *attrWatcher) receive(sock int, done <-chan struct{}) {
	defer unix.Close(sock)
	fd, ok := receiveFd(sock)
	if !ok {
		return
	}
	defer unix.Close(fd)
	w.serve(fd, done)
}

// maxReadFailures is how many reads in a row may fail before the reading is given up (the loop waits 10 ms between two of them).
const maxReadFailures = 20

// streak counts the failures in a row: a read that works begins the count again.
type streak struct{ n int }

func (s *streak) fail() (giveUp bool) { s.n++; return s.n >= maxReadFailures }
func (s *streak) ok()                 { s.n = 0 }

// serve answers the notifications until done is closed.
//
// A failure of reading is tried again, a number of times in a row (maxReadFailures): the descriptor must not be closed at the first one, because the filter
// stays in the program, and once nobody holds the descriptor every call that it hands over fails with ENOSYS. But it cannot be tried for ever either: while it
// fails nothing answers the calls that are waiting, and the program (the linker, with its chmod) waits with them until the run is over by its time. So after
// that many failures in a row the reading is given up, the descriptor is closed (by the caller, when serve returns), the waiting calls and the ones to come fail
// with ENOSYS at once, and the report says so.
func (w *attrWatcher) serve(fd int, done <-chan struct{}) {
	var failures streak
	// retry says whether the loop goes on after a failure: not when the run is over, and not when there have been too many in a row
	retry := func(why string) bool {
		if failures.fail() {
			w.giveUp(why)
			return false
		}
		select {
		case <-done:
			return false
		case <-time.After(10 * time.Millisecond):
			return true
		}
	}
	for {
		select {
		case <-done:
			return
		default:
		}
		pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(pfd, notifPollMillis)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			if !retry("poll: " + err.Error()) {
				return
			}
			continue
		}
		if n == 0 {
			continue
		}
		if pfd[0].Revents&unix.POLLIN == 0 { // POLLHUP: every process that had the filter is gone
			return
		}
		var req notifReq
		if errno := w.receiveOne(fd, &req); errno != 0 {
			if errno == unix.EINTR || errno == unix.ENOENT { // the process went away before the call was read
				continue
			}
			if !retry("receive: " + errno.Error()) {
				return
			}
			continue
		}
		failures.ok()
		w.look(fd, &req)
		// the call goes on as it was asked (the kernel does it): this is a report, not a guard
		resp := notifResp{id: req.id, flags: unix.SECCOMP_USER_NOTIF_FLAG_CONTINUE}
		unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.SECCOMP_IOCTL_NOTIF_SEND, uintptr(unsafe.Pointer(&resp)))
	}
}

// receiveOne reads one notification (a test puts its own function in w.recv).
func (w *attrWatcher) receiveOne(fd int, req *notifReq) unix.Errno {
	if w.recv != nil {
		return w.recv(fd, req)
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.SECCOMP_IOCTL_NOTIF_RECV, uintptr(unsafe.Pointer(req)))
	return errno
}

// giveUp notes that the reading was given up (it goes to the warnings: it is not a change of a file).
func (w *attrWatcher) giveUp(why string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lost = fmt.Sprintf("the reading of the calls that change the attributes of files failed (%s) %d times in a row and was given up: from then on those calls of the program (chmod, chown, utimes, setxattr) fail with ENOSYS, and none of them was looked at", why, maxReadFailures)
}

// warnings are what was not seen.
func (w *attrWatcher) warnings() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lost == "" {
		return nil
	}
	return []string{w.lost}
}

// what pathOf found
const (
	pathOK         = iota // the path is there
	pathUnreadable        // the memory of the process could not be read
	pathNone              // the call is not about a file (an empty path that the kernel refuses)
)

func (w *attrWatcher) look(fd int, req *notifReq) {
	if req.data.arch != seccompArch() {
		return
	}
	var call *attrCall
	for i := range attrCalls {
		if attrCalls[i].nr == uint32(req.data.nr) {
			call = &attrCalls[i]
			break
		}
	}
	if call == nil {
		return
	}
	path, what := w.pathOf(int(req.pid), call, req)
	// the process may have gone and its number may be another process now
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), notifIDValid, uintptr(unsafe.Pointer(&req.id))); errno != 0 {
		return
	}
	switch what {
	case pathNone:
		return
	case pathUnreadable:
		w.mu.Lock()
		w.unread++
		w.mu.Unlock()
		return
	}
	if w.isInside(path) {
		return
	}
	w.add(call.name + " " + path)
}

// pathOf is the file that the call is about, as the program meant it: a relative path is made from the directory of the program (or of the descriptor
// that it gave), a call about a link itself (lchown, AT_SYMLINK_NOFOLLOW) names the link and not what it points to, and an empty path with AT_EMPTY_PATH
// is the descriptor itself. A descriptor that is not a file (a pipe, a socket, a memfd) is nothing to report (pathNone); a path that could not be read is
// pathUnreadable.
func (w *attrWatcher) pathOf(pid int, call *attrCall, req *notifReq) (string, int) {
	arg := func(i int) uint64 { return req.data.args[i] }
	proc := "/proc/" + strconv.Itoa(pid)
	// fdPath is the file of a descriptor of the process
	fdPath := func(fd int) (string, int) {
		p, err := os.Readlink(proc + "/fd/" + strconv.Itoa(fd))
		if err != nil {
			return "", pathUnreadable
		}
		if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "/memfd:") { // pipe:[..], socket:[..], anon_inode:..., a memfd
			return "", pathNone
		}
		return strings.TrimSuffix(p, " (deleted)"), pathOK
	}
	if call.fd >= 0 {
		return fdPath(int(int32(arg(call.fd))))
	}
	flags := 0
	if call.flags >= 0 {
		flags = int(int32(arg(call.flags)))
	}
	nofollow := call.nofollow || flags&atSymlinkNoFoll != 0
	dirfd := atFDCWD
	if call.dirfd >= 0 {
		dirfd = int(int32(arg(call.dirfd)))
	}
	if call.path >= 0 && arg(call.path) == 0 && call.dirfd >= 0 {
		return fdPath(dirfd) // utimensat with no path: the file of the descriptor
	}
	raw, ok := readString(pid, arg(call.path))
	if !ok {
		return "", pathUnreadable
	}
	if raw == "" {
		if flags&atEmptyPath == 0 {
			return "", pathNone // the kernel refuses an empty path
		}
		// the file of the descriptor itself (or the directory of the program, for AT_FDCWD)
		if dirfd == atFDCWD {
			if b, err := os.Readlink(proc + "/cwd"); err == nil {
				return b, pathOK
			}
			return "", pathUnreadable
		}
		return fdPath(dirfd)
	}
	if strings.HasPrefix(raw, "/") {
		return resolve(raw, nofollow), pathOK
	}
	var base string
	if dirfd == atFDCWD {
		b, err := os.Readlink(proc + "/cwd")
		if err != nil {
			return "", pathUnreadable
		}
		base = b
	} else {
		b, what := fdPath(dirfd)
		if what != pathOK {
			return "", what
		}
		base = b
	}
	// (joined as a string: filepath.Join would clean it, and `link/..` would be read as nothing; resolve goes component by component)
	return resolve(base+"/"+raw, nofollow), pathOK
}

// readString reads a string of the memory of a process.
func readString(pid int, addr uint64) (string, bool) {
	f, err := os.Open("/proc/" + strconv.Itoa(pid) + "/mem")
	if err != nil {
		return "", false
	}
	defer f.Close()
	var out []byte
	for len(out) < maxPathLen {
		// up to the end of a block of 256 bytes: a string that ends before an unmapped page is read whole
		n := 256 - int(addr%256)
		buf := make([]byte, n)
		got, err := f.ReadAt(buf, int64(addr))
		if got == 0 && err != nil {
			return "", false
		}
		if i := strings.IndexByte(string(buf[:got]), 0); i >= 0 {
			return string(append(out, buf[:i]...)), true
		}
		out = append(out, buf[:got]...)
		addr += uint64(got)
	}
	return "", false
}

// splitPath is the names of a path, with the empty ones and "." left out (".." stays: what it means depends on the links before it).
func splitPath(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" && c != "." {
			out = append(out, c)
		}
	}
	return out
}

// resolve is the file that the kernel finds for the path: it goes through the names one at a time, follows the links that it meets, and takes ".." from the
// place that it has reached (so `link/..` is the parent of what the link points to, and not the directory that holds the link: a path that is cleaned as a
// string first is read wrong, and a change of a file outside would look like a change inside). With nofollow the last name is not followed (the call is about the
// link itself). A name that is not there ends it: the rest is added as it is (the call fails, and changes nothing).
func resolve(p string, nofollow bool) string {
	const maxLinks = 40
	links := 0
	cur := "/"
	rest := splitPath(p)
	for len(rest) > 0 {
		c := rest[0]
		rest = rest[1:]
		if c == ".." {
			cur = filepath.Dir(cur) // cur has no link in it: its parent is the parent
			continue
		}
		next := filepath.Join(cur, c)
		fi, err := os.Lstat(next)
		if err != nil {
			return filepath.Join(append([]string{next}, rest...)...)
		}
		if fi.Mode()&os.ModeSymlink != 0 && !(len(rest) == 0 && nofollow) {
			links++
			target, err := os.Readlink(next)
			if links > maxLinks || err != nil {
				return next
			}
			if strings.HasPrefix(target, "/") {
				cur = "/"
			}
			rest = append(splitPath(target), rest...)
			continue
		}
		cur = next
	}
	return cur
}

func (w *attrWatcher) isInside(p string) bool {
	for _, d := range w.inside {
		if p == d || strings.HasPrefix(p, strings.TrimSuffix(d, "/")+"/") {
			return true
		}
	}
	return false
}

func (w *attrWatcher) add(s string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.seen[s] {
		return
	}
	w.seen[s] = true
	if len(w.list) >= maxChangesKept {
		w.cut++
		return
	}
	w.list = append(w.list, s)
}

// changes is what was seen (and how much was left out, and what could not be seen).
func (w *attrWatcher) changes() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := append([]string(nil), w.list...)
	if w.cut > 0 {
		out = append(out, fmt.Sprintf("... and %d more", w.cut))
	}
	if w.unread > 0 {
		out = append(out, fmt.Sprintf("(%d more changes of files whose path could not be read: ptrace of the child is not allowed here)", w.unread))
	}
	return out
}
