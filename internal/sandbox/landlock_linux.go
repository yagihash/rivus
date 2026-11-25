//go:build linux

// The sandbox of Linux: Landlock (the files, and the TCP ports) and seccomp (the sockets, and the system calls that go round Landlock),
// with no new privileges. See docs/DESIGN-linux-sandbox.md. The types and the supervision are in sandbox.go.
//
// How it is made. Landlock and seccomp apply to the process that asks for them and to what it starts, and they cannot be taken off. So a
// run starts rivus itself again as a helper (argv[0] is helperName; init below takes it before main): the helper reads what is allowed from a
// pipe, closes itself in, and then execs the program. Nothing of the project runs before the helper has closed itself in, and when it could not
// (no Landlock, no seccomp, a kernel too old for what was asked) it says so on a second pipe and the program is not run (ErrUnavailable).
package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// helperName is argv[0] of the helper. A program that is not the helper never has it.
const helperName = "rivus-sandbox-helper"

// the file descriptors that the helper gets: the configuration to read, and the pipe that says why it could not close itself in.
const (
	fdConfig = 3
	fdStatus = 4
)

// How the network is allowed.
const (
	netNone  = iota // no sockets at all
	netProxy        // TCP to one port
	netOpen         // everything (the fetch stage)
)

// helperConfig is what the helper is told: the paths are real paths (symbolic links resolved) of the caller.
type helperConfig struct {
	// Path is the program (a real path), Argv its arguments (Argv[0] as the caller gave it)
	Path                 string
	Argv, Env            []string
	Read, Exec, Write    []string
	ExecFiles, ReadFiles []string
	Net                  int
	ProxyPort            int
}

func init() {
	if len(os.Args) > 0 && os.Args[0] == helperName {
		helperMain()
	}
}

// ----- the parent -----

// Run runs the program under the sandbox and waits for it.
func Run(ctx context.Context, s Spec) (*Result, error) {
	if len(s.Argv) == 0 || !filepath.IsAbs(s.Argv[0]) {
		return nil, fmt.Errorf("sandbox: the program must be an absolute path: %v", s.Argv)
	}
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("%w: cannot find rivus itself to make the helper: %v", ErrUnavailable, err)
	}
	cfg := helperConfig{Path: real(s.Argv[0]), Argv: append([]string(nil), s.Argv...), Env: s.Env, Net: netNone, ProxyPort: s.ProxyPort}
	for _, d := range s.Read {
		cfg.Read = append(cfg.Read, real(d))
	}
	for _, d := range s.Exec {
		cfg.Exec = append(cfg.Exec, real(d))
	}
	for _, d := range s.Write {
		cfg.Write = append(cfg.Write, real(d))
	}
	for _, f := range append(append([]string(nil), s.ExecFiles...), cfg.Path) {
		cfg.ExecFiles = append(cfg.ExecFiles, real(f))
	}
	for _, f := range s.ReadFiles {
		cfg.ReadFiles = append(cfg.ReadFiles, real(f))
	}
	switch {
	case s.Network:
		cfg.Net = netOpen
	case s.ProxyPort != 0:
		cfg.Net = netProxy
	}
	cfg.addBaseline()
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	cfgR, cfgW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer cfgR.Close()
	stR, stW, err := os.Pipe()
	if err != nil {
		cfgW.Close()
		return nil, err
	}
	defer stR.Close()
	cmd := &exec.Cmd{Path: self, Args: []string{helperName}, ExtraFiles: []*os.File{cfgR, stW}}
	// the helper itself needs nothing from the environment; the program gets s.Env from the configuration
	var msg bytes.Buffer
	msgDone := make(chan struct{})
	res, err := supervise(ctx, s, cmd, func() {
		stW.Close() // only the helper holds the write end now (it is closed on exec)
		go func() { io.Copy(&msg, io.LimitReader(stR, 1<<16)); close(msgDone) }()
		go func() { cfgW.Write(data); cfgW.Close() }()
	})
	if err != nil && res == nil {
		cfgW.Close()
		stW.Close()
		return nil, err
	}
	select {
	case <-msgDone:
	case <-time.After(5 * time.Second):
	}
	if m := strings.TrimSpace(msg.String()); m != "" {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, m)
	}
	return res, err
}

// addBaseline adds what every program needs to start on Linux, with its reason beside it.
func (c *helperConfig) addBaseline() {
	// the libraries of the system: every dynamically linked program reads them. They are READ, not run from: a program may be run only when it is named
	// (the programs of a system can be under /usr/lib too: the Rust coreutils of Ubuntu are in /usr/lib/cargo/bin).
	for _, d := range []string{"/lib", "/lib64", "/usr/lib", "/usr/lib64"} {
		c.Read = append(c.Read, real(d))
	}
	// the dynamic loader is the one thing that the kernel runs for every dynamically linked program: it is run by its path, and only it
	c.ExecFiles = append(c.ExecFiles, loaders()...)
	// /dev/null and the sources of random numbers: the programs write to the one and read from the others
	c.Write = append(c.Write, "/dev/null")
	c.ReadFiles = append(c.ReadFiles, "/dev/zero", "/dev/urandom", "/dev/random", "/etc/ld.so.cache")
	if c.Net == netOpen {
		// name resolution and TLS of the fetch stage
		for _, f := range []string{"/etc/resolv.conf", "/etc/hosts", "/etc/nsswitch.conf", "/etc/host.conf", "/etc/gai.conf"} {
			c.ReadFiles = append(c.ReadFiles, real(f))
		}
		for _, d := range []string{"/etc/ssl", "/etc/ca-certificates", "/etc/pki", "/usr/share/ca-certificates"} {
			c.Read = append(c.Read, real(d))
		}
	}
}

// loaders are the dynamic loaders of the system (real paths).
func loaders() []string {
	var out []string
	for _, g := range []string{"/lib64/ld-linux-*.so.*", "/lib/ld-linux-*.so.*", "/lib/*/ld-linux-*.so.*", "/usr/lib/*/ld-linux-*.so.*", "/usr/lib64/ld-linux-*.so.*", "/lib/ld-musl-*.so.*", "/usr/lib/ld-musl-*.so.*"} {
		ms, _ := filepath.Glob(g)
		for _, m := range ms {
			out = append(out, real(m))
		}
	}
	return out
}

// Available says whether the sandbox can be made on this machine: Landlock and seccomp are there and accept what a run asks for (a
// program that does nothing is run under it once). A caller asks before it runs the code of a project, and never takes what
// the project's program writes to its standard error for an answer.
func Available() error {
	truePath, err := exec.LookPath("true")
	if err != nil {
		truePath = "/usr/bin/true"
	}
	truePath, _ = filepath.Abs(truePath)
	res, err := Run(context.Background(), Spec{Argv: []string{truePath}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Timeout: 30 * time.Second})
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("%w: the sandbox of a run was not accepted (a program that does nothing exited %d): %s", ErrUnavailable, res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}

// ABI says which version of Landlock this kernel has (0: none, or it is turned off).
func ABI() int {
	v, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return 0
	}
	return int(v)
}

// ----- the helper -----

// fail says why the sandbox could not be made, on the status pipe, and ends. Nothing of the project has run.
func fail(format string, a ...any) {
	st := os.NewFile(fdStatus, "status")
	fmt.Fprintf(st, format, a...)
	st.Close()
	os.Exit(125)
}

func helperMain() {
	// the state of Landlock and seccomp belongs to the thread: the thread that closes itself in is the one that execs
	runtime.LockOSThread()
	unix.CloseOnExec(fdConfig)
	unix.CloseOnExec(fdStatus)
	var cfg helperConfig
	cf := os.NewFile(fdConfig, "config")
	if err := json.NewDecoder(cf).Decode(&cfg); err != nil {
		fail("reading what the sandbox is to allow: %v", err)
	}
	cf.Close()
	if err := closeIn(&cfg); err != nil {
		fail("%v", err)
	}
	// from here on, the program is what runs; the status pipe is closed by the exec (it was made CLOEXEC), which is the answer "all well"
	if err := syscall.Exec(cfg.Path, cfg.Argv, cfg.Env); err != nil {
		fail("exec %s: %v", cfg.Path, err)
	}
}

// closeIn makes this thread unable to do what the sandbox forbids.
func closeIn(cfg *helperConfig) error {
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("no_new_privs: %v", err)
	}
	dropCapabilities()
	if err := landlock(cfg); err != nil {
		return err
	}
	return seccomp(cfg)
}

// dropCapabilities takes every capability away from what is run (it matters when rivus is run as root, or with capabilities: no_new_privs does not
// take them away, and root gets all of them again at an exec). The bounding set is emptied, the "root has them" rule of exec is turned off (and
// locked), the ambient and inheritable sets are cleared. Each step is best effort: a process that has no capability has nothing to drop, and
// the steps that need one (CAP_SETPCAP) are refused for it.
func dropCapabilities() {
	unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_CLEAR_ALL, 0, 0, 0)
	for c := 0; c < 64; c++ {
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, uintptr(c), 0, 0, 0); err == unix.EINVAL {
			break // past the last capability that this kernel has
		}
	}
	// SECBIT_NOROOT | SECBIT_NOROOT_LOCKED | SECBIT_NO_CAP_AMBIENT_RAISE | SECBIT_NO_CAP_AMBIENT_RAISE_LOCKED
	unix.Prctl(unix.PR_SET_SECUREBITS, 0x1|0x2|0x40|0x80, 0, 0, 0)
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	unix.Capset(&hdr, &data[0])
}

// ----- Landlock -----

const (
	ruleTypePath = 1 // LANDLOCK_RULE_PATH_BENEATH
	ruleTypeNet  = 2 // LANDLOCK_RULE_NET_PORT
)

type rulesetAttr struct {
	fs, net, scoped uint64
}

type pathBeneath struct {
	allowed uint64
	fd      int32
	_       int32
}

type netPort struct {
	allowed, port uint64
}

// the access rights of a directory and of a file (the rights of a directory that a file cannot have are refused by the kernel)
const (
	fsDirOnly = unix.LANDLOCK_ACCESS_FS_READ_DIR | unix.LANDLOCK_ACCESS_FS_REMOVE_DIR | unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
		unix.LANDLOCK_ACCESS_FS_MAKE_CHAR | unix.LANDLOCK_ACCESS_FS_MAKE_DIR | unix.LANDLOCK_ACCESS_FS_MAKE_REG |
		unix.LANDLOCK_ACCESS_FS_MAKE_SOCK | unix.LANDLOCK_ACCESS_FS_MAKE_FIFO | unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_SYM | unix.LANDLOCK_ACCESS_FS_REFER
	fsRead  = unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_READ_DIR
	fsExec  = fsRead | unix.LANDLOCK_ACCESS_FS_EXECUTE
	fsWrite = fsExec | unix.LANDLOCK_ACCESS_FS_WRITE_FILE | unix.LANDLOCK_ACCESS_FS_REMOVE_DIR | unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
		unix.LANDLOCK_ACCESS_FS_MAKE_CHAR | unix.LANDLOCK_ACCESS_FS_MAKE_DIR | unix.LANDLOCK_ACCESS_FS_MAKE_REG |
		unix.LANDLOCK_ACCESS_FS_MAKE_SOCK | unix.LANDLOCK_ACCESS_FS_MAKE_FIFO | unix.LANDLOCK_ACCESS_FS_MAKE_SYM |
		unix.LANDLOCK_ACCESS_FS_REFER | unix.LANDLOCK_ACCESS_FS_TRUNCATE
)

// handledFS are the rights of files that this kernel knows (every one of them is denied unless a rule gives it).
func handledFS(abi int) uint64 {
	h := uint64(unix.LANDLOCK_ACCESS_FS_EXECUTE | unix.LANDLOCK_ACCESS_FS_WRITE_FILE | unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_DIR | unix.LANDLOCK_ACCESS_FS_REMOVE_DIR | unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
		unix.LANDLOCK_ACCESS_FS_MAKE_CHAR | unix.LANDLOCK_ACCESS_FS_MAKE_DIR | unix.LANDLOCK_ACCESS_FS_MAKE_REG |
		unix.LANDLOCK_ACCESS_FS_MAKE_SOCK | unix.LANDLOCK_ACCESS_FS_MAKE_FIFO | unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_SYM)
	if abi >= 2 {
		h |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi >= 3 {
		h |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	return h
}

func landlock(cfg *helperConfig) error {
	abi := ABI()
	if abi < 1 {
		return fmt.Errorf("Landlock is not there: the kernel has none, or it is turned off (it is in the kernel since 5.13; `cat /sys/kernel/security/lsm` should list it)")
	}
	if cfg.Net == netProxy && abi < 4 {
		return fmt.Errorf("the TCP port of the proxy can be the only one to connect to only with Landlock ABI 4 (Linux 6.7); this kernel has ABI %d", abi)
	}
	at := rulesetAttr{fs: handledFS(abi)}
	if cfg.Net != netOpen && abi >= 4 {
		// belt and braces for what seccomp does: no bind, and no connect except to the port that a rule names
		at.net = unix.LANDLOCK_ACCESS_NET_BIND_TCP | unix.LANDLOCK_ACCESS_NET_CONNECT_TCP
	}
	if abi >= 6 {
		// no signal to a process outside, and no abstract UNIX socket of one outside
		at.scoped = unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET | unix.LANDLOCK_SCOPE_SIGNAL
	}
	rfd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&at)), unsafe.Sizeof(at), 0)
	if errno != 0 {
		return fmt.Errorf("Landlock: creating the rules: %v", errno)
	}
	defer unix.Close(int(rfd))
	handled := at.fs
	add := func(paths []string, access uint64) error {
		for _, p := range paths {
			if err := addPath(int(rfd), p, access&handled); err != nil {
				return err
			}
		}
		return nil
	}
	if err := add(cfg.Read, fsRead); err != nil {
		return err
	}
	if err := add(cfg.Exec, fsExec); err != nil {
		return err
	}
	if err := add(cfg.ExecFiles, fsExec); err != nil {
		return err
	}
	if err := add(cfg.ReadFiles, fsRead); err != nil {
		return err
	}
	// the working directory of the run: written, and programs run from it (the build scripts and the proc macros that the build makes there)
	if err := add(cfg.Write, fsWrite); err != nil {
		return err
	}
	if cfg.Net == netProxy {
		np := netPort{allowed: unix.LANDLOCK_ACCESS_NET_CONNECT_TCP, port: uint64(cfg.ProxyPort)}
		if _, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, rfd, ruleTypeNet, uintptr(unsafe.Pointer(&np)), 0, 0, 0); errno != 0 {
			return fmt.Errorf("Landlock: the port %d: %v", cfg.ProxyPort, errno)
		}
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, rfd, 0, 0); errno != 0 {
		return fmt.Errorf("Landlock: closing in: %v", errno)
	}
	return nil
}

// addPath gives the rights to the path and what is below it. A path that is not there is left out (the caller names the places that a
// system may or may not have): what is not allowed is denied.
func addPath(rfd int, p string, access uint64) error {
	if access == 0 {
		return nil
	}
	fd, err := unix.Open(p, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		if err == unix.ENOENT || err == unix.ENOTDIR || err == unix.EACCES {
			return nil
		}
		return fmt.Errorf("Landlock: %s: %v", p, err)
	}
	defer unix.Close(fd)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return fmt.Errorf("Landlock: %s: %v", p, err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		access &^= fsDirOnly
	}
	pb := pathBeneath{allowed: access, fd: int32(fd)}
	if _, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, uintptr(rfd), ruleTypePath, uintptr(unsafe.Pointer(&pb)), 0, 0, 0); errno != 0 {
		return fmt.Errorf("Landlock: %s: %v", p, errno)
	}
	return nil
}
