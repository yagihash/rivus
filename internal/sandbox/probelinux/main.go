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
	// ptrace (a request that does nothing: only the refusal of the call itself is told from the rest)
	_, _, errno = unix.Syscall6(unix.SYS_PTRACE, 0xffff, 0, 0, 0, 0, 0)
	report("ptrace", errno != unix.EPERM, fmt.Sprint(errno))
}
