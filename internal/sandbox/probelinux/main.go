//go:build linux

// Command probelinux tries, one by one, what a hostile build script might do to get out of the sandbox on Linux, and says for each whether it
// got out (ESCAPED) or not (BLOCKED). It is the stand-in of the hostile crate for the tests of the sandbox (docs/DESIGN-linux-sandbox.md): run
// on the host it must say ESCAPED for the checks that the test set up (the holes are real), and under the sandbox BLOCKED for all.
//
// Everything is harmless: the evidence is a fake secret file, listeners that the test started, an empty file in a directory of the test.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var (
	canary = flag.String("canary", "", "a file with a fake secret that may not be read")
	dir    = flag.String("dir", "", "a directory that may not be written")
	tcp    = flag.String("tcp", "", "host:port of a TCP listener of the test")
	tcp2   = flag.String("tcp2", "", "host:port of another TCP listener of the test")
	sock   = flag.String("sock", "", "a UNIX socket (a path) that the test listens on")
	udp    = flag.String("udp", "", "host:port of a UDP listener of the test")
	prog   = flag.String("exec", "", "a program that is not allowed to be run")
	only   = flag.String("only", "", "run only this check")
	attr   = flag.String("attr", "", "a file outside of the places that may be written: its mode and its times are changed (Landlock does not stop that; the parent reports it)")
	attrIn = flag.String("attr-in", "", "a file in the work directory: its mode is changed (not reported)")
	attrLn = flag.String("attr-link", "", "a symbolic link in the work directory that points outside: lchown of the link itself (the link is inside: not reported; what it points to is not touched)")
	inhFd  = flag.Int("inherited-fd", -1, "a file descriptor that the test left open in the process that started the probe (not close-on-exec): is it still here?")
	attrDD = flag.String("attr-dotdot", "", "a path, as it is written (not cleaned), whose mode is changed: dir/link/../file, where link points to another directory")
	attrNl = flag.Bool("attr-null", false, "the mode of /dev/null is changed (a place that the run may write)")
	attrFd = flag.String("attr-fd", "", "a file that may be read and not written: it is opened, and fchownat with AT_EMPTY_PATH is made on the descriptor (reported, with the path of the file)")
)

func report(name string, escaped bool, how string) {
	if *only != "" && *only != name {
		return
	}
	s := "BLOCKED"
	if escaped {
		s = "ESCAPED"
	}
	fmt.Printf("%s %s %s\n", name, s, how)
}

func main() {
	flag.Parse()
	if *attr != "" {
		fmt.Println("attr-chmod", os.Chmod(*attr, 0o640))
		fmt.Println("attr-utimes", os.Chtimes(*attr, time.Unix(1, 0), time.Unix(1, 0)))
	}
	if *attrDD != "" {
		fmt.Println("attr-dotdot-chmod", os.Chmod(*attrDD, 0o640))
	}
	if *attrNl {
		fmt.Println("attr-null-chmod", os.Chmod("/dev/null", 0o666))
	}
	if *attrLn != "" {
		fmt.Println("attr-link-lchown", os.Lchown(*attrLn, os.Getuid(), os.Getgid()))
	}
	if *attrFd != "" {
		f, err := os.Open(*attrFd)
		if err != nil {
			fmt.Println("attr-fd-open", err)
		} else {
			fmt.Println("attr-fd-fchownat", unix.Fchownat(int(f.Fd()), "", os.Getuid(), os.Getgid(), unix.AT_EMPTY_PATH))
			f.Close()
		}
	}
	if *attrIn != "" {
		fmt.Println("attr-in-chmod", os.Chmod(*attrIn, 0o640))
	}
	if *canary != "" {
		b, err := os.ReadFile(*canary)
		report("read-secret", err == nil, fmt.Sprint(err, len(b)))
	}
	if *inhFd >= 0 {
		_, err := unix.FcntlInt(uintptr(*inhFd), unix.F_GETFD, 0)
		report("inherited-fd", err == nil, fmt.Sprint(err))
	}
	if *dir != "" {
		f, err := os.Create(*dir + "/rivus-sbx-probe")
		if err == nil {
			f.Close()
			os.Remove(*dir + "/rivus-sbx-probe")
		}
		report("write-outside", err == nil, fmt.Sprint(err))
	}
	if *tcp != "" {
		c, err := net.DialTimeout("tcp", *tcp, 2*time.Second)
		if err == nil {
			c.Close()
		}
		report("tcp", err == nil, fmt.Sprint(err))
	}
	if *tcp2 != "" {
		c, err := net.DialTimeout("tcp", *tcp2, 2*time.Second)
		if err == nil {
			c.Close()
		}
		report("tcp-other-port", err == nil, fmt.Sprint(err))
	}
	if *udp != "" {
		c, err := net.DialTimeout("udp", *udp, 2*time.Second)
		if err == nil {
			_, err = c.Write([]byte("x"))
			c.Close()
		}
		report("udp", err == nil, fmt.Sprint(err))
	}
	if *sock != "" {
		c, err := net.DialTimeout("unix", *sock, 2*time.Second)
		if err == nil {
			c.Close()
		}
		report("unix-socket", err == nil, fmt.Sprint(err))
	}
	if *prog != "" {
		err := exec.Command(*prog).Run()
		_, isExit := err.(*exec.ExitError)
		report("exec-other", err == nil || isExit, fmt.Sprint(err))
	}
	// the memory and the environment of a process of the user: the parent is one
	_, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", os.Getppid()))
	report("proc-environ", err == nil, fmt.Sprint(err))
	// a signal to a process outside
	err = syscall.Kill(os.Getppid(), 0)
	report("signal-outside", err == nil, fmt.Sprint(err))
	// the sockets of TCP-like protocols that the port rule of Landlock does not cover (it knows TCP only): SCTP and MPTCP
	for _, pr := range []struct {
		name  string
		proto int
	}{{"sctp", 132}, {"mptcp", 262}} {
		fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, pr.proto)
		if err == nil {
			unix.Close(fd)
		}
		report(pr.name, err != unix.EPERM, fmt.Sprint(err))
	}
	// the IPC of the system that is not mediated by Landlock and is shared with the other processes of the user (the IPC namespace is the host's): SysV
	// shared memory, message queues and semaphores, POSIX message queues, kcmp. Arguments that fail at once on the host (no such key, no name, no such process)
	// tell the refusal (EPERM) from the rest, and create nothing.
	for _, c := range []struct {
		name string
		nr   uintptr
		args [3]uintptr
	}{
		{"sysv-shm", unix.SYS_SHMGET, [3]uintptr{0x7fff0001, 0, 0}},
		{"sysv-msg", unix.SYS_MSGGET, [3]uintptr{0x7fff0002, 0, 0}},
		{"sysv-sem", unix.SYS_SEMGET, [3]uintptr{0x7fff0003, 0, 0}},
		{"posix-mq", unix.SYS_MQ_OPEN, [3]uintptr{0, 0, 0}},
		{"kcmp", unix.SYS_KCMP, [3]uintptr{0, 0, 0}},
	} {
		_, _, errno := unix.Syscall(c.nr, c.args[0], c.args[1], c.args[2])
		report(c.name, errno != unix.EPERM, fmt.Sprint(errno))
	}
	// io_uring (it makes sockets by itself)
	_, _, errno := unix.Syscall(unix.SYS_IO_URING_SETUP, 1, 0, 0)
	report("io-uring", errno != unix.EPERM, fmt.Sprint(errno))
	// a new user namespace: the call is refused before the kernel looks at the arguments, so the arguments that make it fail anyway (the
	// threads of this process) tell the refusal (EPERM) from the rest
	_, _, errno = unix.Syscall(unix.SYS_UNSHARE, unix.CLONE_NEWUSER, 0, 0)
	report("unshare", errno != unix.EPERM, fmt.Sprint(errno))
	_, _, errno = unix.Syscall6(unix.SYS_CLONE, unix.CLONE_NEWNS|unix.CLONE_THREAD, 0, 0, 0, 0, 0)
	report("clone-newns", errno != unix.EPERM, fmt.Sprint(errno))
	// clone3 is said not to be there (ENOSYS); on the host the call with no arguments fails in another way
	_, _, errno = unix.Syscall(unix.SYS_CLONE3, 0, 0, 0)
	report("clone3", errno != unix.ENOSYS, fmt.Sprint(errno))
	// the rights of root (only when this is run as root: for anyone else the file cannot be given away either way)
	if os.Geteuid() == 0 {
		var hdr = unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
		var data [2]unix.CapUserData
		unix.Capget(&hdr, &data[0])
		report("capabilities", data[0].Permitted|data[1].Permitted|data[0].Effective|data[1].Effective != 0, fmt.Sprintf("%x %x", data[0].Effective, data[1].Effective))
		if *canary != "" {
			err := os.Chown(*canary, 12345, 12345)
			report("chown", err == nil, fmt.Sprint(err))
		}
	}
	// ptrace (a request that does nothing: only the refusal of the call itself is told from the rest)
	_, _, errno = unix.Syscall6(unix.SYS_PTRACE, 0xffff, 0, 0, 0, 0, 0)
	report("ptrace", errno != unix.EPERM, fmt.Sprint(errno))
}
