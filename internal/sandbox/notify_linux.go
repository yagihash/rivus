//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Landlock does not stop the calls that change the attributes of a file (the mode, the owner, the times, the extended attributes): a program could change the
// mode of a file of the same user that it may not read or write. A filter of seccomp cannot look at a path, and a call that is refused outright breaks the linker (it
// sets the mode of its output with chmod). So these calls (attrCalls) are handed to rivus, which does them FOR the program, and only for the files inside of the places
// that the run may write (the work directory): for a file outside the answer is EPERM, and the refusal is reported.
//
// The call is not let through (SECCOMP_USER_NOTIF_FLAG_CONTINUE would make the kernel read the path again from the memory of the program, and another thread of the
// program can change it between the two reads: the check would be of one file and the change of another; seccomp_unotify(2) says so). What is done is what rivus
// copied: the path and the other arguments are read once from the memory of the program; the file is opened once (O_PATH), the file that was opened is the one that is
// checked (its path from /proc/self/fd, not a string that can be read again) and the one that the change is made on. A call on a descriptor takes the descriptor of the
// program by pidfd_getfd, so that the file cannot be changed under it by a dup2 of another thread.

// attrOp is what a call does to the file.
type attrOp int

const (
	opChmod attrOp = iota + 1
	opChown
	opUtimes
	opSetxattr
	opRemovexattr
)

// attrCall says what a system call that changes attributes is and where its arguments are (-1: it has none of that kind).
type attrCall struct {
	nr       uint32
	name     string
	op       attrOp
	fd       int  // the file descriptor of the file
	dirfd    int  // the directory that path is relative to
	path     int  // the path (a string)
	flags    int  // the flags of the call (AT_SYMLINK_NOFOLLOW, AT_EMPTY_PATH)
	nofollow bool // the call is about the link itself, not about what it points to (lchown, lsetxattr, lremovexattr)
	// the arguments of the operation
	mode, uid, gid, times, xname, value, size, xflags int
}

func newCall(nr uint32, name string, op attrOp) attrCall {
	return attrCall{nr: nr, name: name, op: op, fd: -1, dirfd: -1, path: -1, flags: -1, mode: -1, uid: -1, gid: -1, times: -1, xname: -1, value: -1, size: -1, xflags: -1}
}

// the forms of the calls: by a path (argument 0), by a descriptor (argument 0), and relative to a directory (the descriptor of the directory is argument 0 and the
// path argument 1; flags is the number of the argument that holds AT_* flags, -1 when there is none)
func chmodPath(nr uint32, name string) attrCall {
	c := newCall(nr, name, opChmod)
	c.path, c.mode = 0, 1
	return c
}

func chmodFd(nr uint32, name string) attrCall {
	c := newCall(nr, name, opChmod)
	c.fd, c.mode = 0, 1
	return c
}

func chmodAt(nr uint32, name string, flags int) attrCall {
	c := newCall(nr, name, opChmod)
	c.dirfd, c.path, c.mode, c.flags = 0, 1, 2, flags
	return c
}

func chownPath(nr uint32, name string, nofollow bool) attrCall {
	c := newCall(nr, name, opChown)
	c.path, c.uid, c.gid, c.nofollow = 0, 1, 2, nofollow
	return c
}

func chownFd(nr uint32, name string) attrCall {
	c := newCall(nr, name, opChown)
	c.fd, c.uid, c.gid = 0, 1, 2
	return c
}

func chownAt(nr uint32, name string) attrCall {
	c := newCall(nr, name, opChown)
	c.dirfd, c.path, c.uid, c.gid, c.flags = 0, 1, 2, 3, 4
	return c
}

func utimensat(nr uint32) attrCall {
	c := newCall(nr, "utimensat", opUtimes)
	c.dirfd, c.path, c.times, c.flags = 0, 1, 2, 3
	return c
}

func setxattrPath(nr uint32, name string, nofollow bool) attrCall {
	c := newCall(nr, name, opSetxattr)
	c.path, c.xname, c.value, c.size, c.xflags, c.nofollow = 0, 1, 2, 3, 4, nofollow
	return c
}

func setxattrFd(nr uint32, name string) attrCall {
	c := newCall(nr, name, opSetxattr)
	c.fd, c.xname, c.value, c.size, c.xflags = 0, 1, 2, 3, 4
	return c
}

func removexattrPath(nr uint32, name string, nofollow bool) attrCall {
	c := newCall(nr, name, opRemovexattr)
	c.path, c.xname, c.nofollow = 0, 1, nofollow
	return c
}

func removexattrFd(nr uint32, name string) attrCall {
	c := newCall(nr, name, opRemovexattr)
	c.fd, c.xname = 0, 1
	return c
}

const (
	atFDCWD         = -100
	atSymlinkNoFoll = 0x100
	atEmptyPath     = 0x1000
	notifIDValid    = 0x40082102 // SECCOMP_IOCTL_NOTIF_ID_VALID (the same on x86-64 and arm64)
	maxChangesKept  = 200
	maxPathLen      = 4096
	maxXattrValue   = 65536 // XATTR_SIZE_MAX
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
	errno int32 // minus the errno of the call; 0 when it worked
	flags uint32
}

// attrWatcher serves the notifications of a run: it does the calls that are inside of the places that the run may write, and refuses the others.
type attrWatcher struct {
	inside []string // real paths of the directories that the run may write
	mu     sync.Mutex
	seen   map[string]bool
	list   []string
	cut    int
	unread int    // the calls that were refused because the file or the arguments could not be read
	lost   string // the reading of the notifications was given up
	noAct  string // the thread that does the calls could not give up its capabilities: nothing is done
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
//
// The thread that serves does the calls for the program, and it must be able to do no more than the program: the program has no capability (dropCapabilities), and
// if rivus is run as root or with capabilities, the thread gives them up first (a capability is a property of a thread: only this one is changed, and it is kept by this
// goroutine until the end, when the thread ends with it). If it cannot, nothing is done for the program and every call is refused.
func (w *attrWatcher) receive(sock int, done <-chan struct{}) {
	runtime.LockOSThread()
	defer unix.Close(sock)
	fd, ok := receiveFd(sock)
	if !ok {
		return
	}
	defer unix.Close(fd)
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capset(&hdr, &data[0]); err != nil {
		w.mu.Lock()
		w.noAct = "the thread that does the calls of the program could not give up its capabilities (" + err.Error() + ")"
		w.mu.Unlock()
	}
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
		if resp, send := w.handle(fd, &req); send {
			unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.SECCOMP_IOCTL_NOTIF_SEND, uintptr(unsafe.Pointer(&resp)))
		}
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
	w.lost = fmt.Sprintf("the reading of the calls that change the attributes of files failed (%s) %d times in a row and was given up: from then on those calls of the program (chmod, chown, utimes, setxattr) fail with ENOSYS, and none of them was done or looked at", why, maxReadFailures)
}

// handle answers one call. The answer is what the program gets: 0 when the call was done for it, the errno of the call when it failed, EPERM when it was refused
// (a file outside of the places that may be written, a file or arguments that could not be read, a form of the call that is not done). send is false when
// the program is gone (nothing is waiting).
func (w *attrWatcher) handle(fd int, req *notifReq) (notifResp, bool) {
	resp := notifResp{id: req.id}
	refuse := func() (notifResp, bool) { resp.errno = -int32(unix.EPERM); return resp, true }
	if req.data.arch != seccompArch() {
		return refuse()
	}
	var call *attrCall
	for i := range attrCalls {
		if attrCalls[i].nr == uint32(req.data.nr) {
			call = &attrCalls[i]
			break
		}
	}
	if call == nil {
		return refuse()
	}
	w.mu.Lock()
	noAct := w.noAct
	w.mu.Unlock()
	if noAct != "" {
		w.countUnread()
		return refuse()
	}
	out := w.perform(fd, req, call)
	if out.gone {
		return resp, false
	}
	if out.unread {
		w.countUnread()
	}
	if out.refused != "" {
		w.add(out.refused)
	}
	resp.errno = -int32(out.errno)
	return resp, true
}

func (w *attrWatcher) countUnread() {
	w.mu.Lock()
	w.unread++
	w.mu.Unlock()
}

// idValid says whether the call is still waiting (the process may have gone, and its number may be another process now).
func idValid(fd int, id uint64) bool {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), notifIDValid, uintptr(unsafe.Pointer(&id)))
	return errno == 0
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

// readBytes reads n bytes of the memory of a process.
func readBytes(pid int, addr uint64, n int) ([]byte, bool) {
	if n == 0 {
		return nil, true
	}
	f, err := os.Open("/proc/" + strconv.Itoa(pid) + "/mem")
	if err != nil {
		return nil, false
	}
	defer f.Close()
	buf := make([]byte, n)
	got, _ := f.ReadAt(buf, int64(addr))
	if got != n {
		return nil, false
	}
	return buf, true
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

// changes are the calls that were refused (a file outside of the places that may be written), and how much was left out.
func (w *attrWatcher) changes() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := append([]string(nil), w.list...)
	if w.cut > 0 {
		out = append(out, fmt.Sprintf("... and %d more", w.cut))
	}
	return out
}

// warnings are what could not be done or seen: the calls that were refused because their file could not be told (rivus may not read the memory of the program: ptrace of a
// child that is not allowed), the thread that could not give up its capabilities, and the reading that was given up. They are not refusals of a file, and are not listed as such.
func (w *attrWatcher) warnings() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	if w.unread > 0 {
		out = append(out, fmt.Sprintf("%d calls of the build that change the attributes of files were refused (EPERM), since the file that they name or their arguments could not be read (rivus may not read the memory of the program here: ptrace of a child is not allowed)", w.unread))
	}
	if w.noAct != "" {
		out = append(out, w.noAct+": the calls that change the attributes of files were all refused")
	}
	if w.lost != "" {
		out = append(out, w.lost)
	}
	return out
}
