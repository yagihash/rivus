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
	if *canary != "" {
		b, err := os.ReadFile(*canary)
		report("read-secret", err == nil, fmt.Sprint(err, len(b)))
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
