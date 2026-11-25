//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The calls that change the attributes of a file (attrCalls) are not stopped by Landlock: a program can change the mode of a file of the same user
// that it may not read or write. They are not refused here either (a linker changes the mode of its output with chmod; a path cannot be looked at by a
// filter of seccomp), but they are handed to rivus, which looks at the path and reports the ones that are outside of the places that the run may write.
// The report is after the fact and it is not a guard: what is told is what the program asked for.

// attrCall says where the arguments of a system call that changes attributes are (-1: there is none of that kind).
type attrCall struct {
	nr    uint32
	name  string
	fd    int // the file descriptor of the file
	dirfd int // the directory that path is relative to
	path  int // the path (a string)
}

const (
	atFDCWD         = -100
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
	unread int // the calls whose path could not be read
}

func newAttrWatcher(write []string) *attrWatcher {
	return &attrWatcher{inside: write, seen: map[string]bool{}}
}

// receive reads the descriptor of the notifications that the helper sends over the socket (it sends nothing and closes it when there is none), and
// serves it until done is closed.
func (w *attrWatcher) receive(sock int, done <-chan struct{}) {
	defer unix.Close(sock)
	buf := make([]byte, 1)
	oob := make([]byte, unix.CmsgSpace(4))
	n, oobn, _, _, err := unix.Recvmsg(sock, buf, oob, 0)
	if err != nil || (n == 0 && oobn == 0) {
		return
	}
	msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil || len(msgs) == 0 {
		return
	}
	fds, err := unix.ParseUnixRights(&msgs[0])
	if err != nil || len(fds) == 0 {
		return
	}
	fd := fds[0]
	defer unix.Close(fd)
	w.serve(fd, done)
}

func (w *attrWatcher) serve(fd int, done <-chan struct{}) {
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
			return
		}
		if n == 0 {
			continue
		}
		if pfd[0].Revents&unix.POLLIN == 0 { // POLLHUP: every process that had the filter is gone
			return
		}
		var req notifReq
		if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.SECCOMP_IOCTL_NOTIF_RECV, uintptr(unsafe.Pointer(&req))); errno != 0 {
			if errno == unix.EINTR || errno == unix.ENOENT {
				continue
			}
			return
		}
		w.look(fd, &req)
		// the call goes on as it was asked (the kernel does it): this is a report, not a guard
		resp := notifResp{id: req.id, flags: unix.SECCOMP_USER_NOTIF_FLAG_CONTINUE}
		unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.SECCOMP_IOCTL_NOTIF_SEND, uintptr(unsafe.Pointer(&resp)))
	}
}

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
	pid := int(req.pid)
	path, ok := w.pathOf(pid, call, req)
	// the process may have gone and its number may be another process now
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), notifIDValid, uintptr(unsafe.Pointer(&req.id))); errno != 0 {
		return
	}
	if !ok {
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

// pathOf is the path that the call is about, as the program meant it (relative paths are made from the directory of the program).
func (w *attrWatcher) pathOf(pid int, call *attrCall, req *notifReq) (string, bool) {
	arg := func(i int) uint64 { return req.data.args[i] }
	fdPath := func(fd int) (string, bool) {
		p, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/fd/" + strconv.Itoa(fd))
		if err != nil || !strings.HasPrefix(p, "/") {
			return "", false // a pipe, a socket, and so on
		}
		return strings.TrimSuffix(p, " (deleted)"), true
	}
	if call.fd >= 0 {
		return fdPath(int(int32(arg(call.fd))))
	}
	if call.path >= 0 && arg(call.path) == 0 && call.dirfd >= 0 {
		return fdPath(int(int32(arg(call.dirfd)))) // utimensat with no path: the file of the descriptor
	}
	raw, ok := readString(pid, arg(call.path))
	if !ok {
		return "", false
	}
	if strings.HasPrefix(raw, "/") {
		return resolveLoose(raw), true
	}
	base := ""
	dirfd := atFDCWD
	if call.dirfd >= 0 {
		dirfd = int(int32(arg(call.dirfd)))
	}
	if dirfd == atFDCWD {
		b, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
		if err != nil {
			return "", false
		}
		base = b
	} else {
		b, ok := fdPath(dirfd)
		if !ok {
			return "", false
		}
		base = b
	}
	return resolveLoose(filepath.Join(base, raw)), true
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
		// up to the end of the page: a string that ends in the page before an unmapped one is read whole
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

// resolveLoose makes the path absolute and without symbolic links as far as it exists.
func resolveLoose(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if r, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		return filepath.Join(r, filepath.Base(p))
	}
	return p
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

// changes is what was seen (and how much was left out).
func (w *attrWatcher) changes() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := append([]string(nil), w.list...)
	if w.cut > 0 {
		out = append(out, fmt.Sprintf("... and %d more", w.cut))
	}
	if w.unread > 0 {
		out = append(out, fmt.Sprintf("(%d calls whose path could not be read: ptrace of the child is not allowed here)", w.unread))
	}
	return out
}
