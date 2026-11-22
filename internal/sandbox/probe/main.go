//go:build darwin

// Command probe tries, one by one, what a hostile build script might do to get out of a sandbox on macOS, and says for each
// whether it got out (ESCAPED) or not (BLOCKED). It is the stand-in of the hostile crate for the tests of the sandbox
// (docs/DESIGN-macos-sandbox.md, 3.9): run on the host it must report ESCAPED for the checks of the red set (the
// holes are real), and run under the sandbox it must report BLOCKED for all.
//
// Everything it does is harmless: the evidence is a fake secret file, a listener that the test started, a name that
// the test made unique (`rivus-sbx-<id>`), an empty file. It reads nothing real (it only asks whether a real path
// exists), and it sends nothing outside this machine. The test cleans up what it made.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

var (
	id        = flag.String("id", "", "the unique id of this run (names are rivus-sbx-<id>)")
	dir       = flag.String("dir", "", "a directory of the test that the build may not write to")
	canary    = flag.String("canary", "", "a file with a fake secret that the build may not read")
	tcp6      = flag.String("tcp6", "", "[::1]:port of a listener that the test started on the IPv6 loopback")
	tcp       = flag.String("tcp", "", "host:port of a listener that the test started (loopback)")
	lan       = flag.String("lan", "", "host:port of a listener on the address of this machine on the LAN")
	sock      = flag.String("sock", "", "a UNIX domain socket that the test listens on")
	pid       = flag.Int("pid", 0, "the pid of a process that the test started (it holds FAKE_TOKEN in its environment)")
	token     = flag.String("token", "", "the fake token")
	tool      = flag.String("toolchain", "", "a directory of the toolchain that the build may not write to")
	udp       = flag.String("udp", "", "host:port of a UDP listener that the test started")
	sensitive = flag.Bool("sensitive", false, "also run the checks that touch what is private to the user (the pasteboard, the keychain): only under the sandbox, where they must fail; never without it")
	targets   = flag.String("targets", "", "label=pid,label=pid: processes of the machine whose arguments and environment the probe asks for (kern.procargs2); only whether it could read them is said, never what they hold")
	homeFlag  = flag.String("home", "", "the real home directory of the user (default: $HOME, which a test may have set to a work directory)")
	child     = flag.Bool("child", false, "(internal) setsid, then sleep: the process that is left behind")
	daemon    = flag.Bool("daemon", false, "also leave a process behind (setsid and a double fork), marked with the id")
	only      = flag.String("only", "", "comma separated names of the checks to run (default: all)")
	fakeEnv   = "RIVUS_SBX_FAKE_TOKEN"
	reported  = map[string]bool{}
)

// home and mark are package variables, not locals of main: the checks below are closures, and a closure that captures a local
// is one more that the analysis of rivus by itself counts (TestDogfood).
var home, mark string

func report(name string, escaped bool, detail string) {
	state := "BLOCKED"
	if escaped {
		state = "ESCAPED"
	}
	if detail != "" {
		detail = " " + strings.ReplaceAll(detail, "\n", " ")
	}
	fmt.Printf("%s %s%s\n", state, name, detail)
	reported[name] = true
}

func wanted(name string) bool {
	if *only == "" {
		return true
	}
	for _, n := range strings.Split(*only, ",") {
		if n == name {
			return true
		}
	}
	return false
}

func check(name string, f func() (bool, string)) {
	if !wanted(name) {
		return
	}
	if (name == "pbpaste" || name == "securityd_reachable") && !*sensitive {
		return
	}
	escaped, detail := f()
	report(name, escaped, detail)
}

func run(timeout time.Duration, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = nil
	done := make(chan struct{})
	var out []byte
	var err error
	go func() { out, err = cmd.CombinedOutput(); close(done) }()
	select {
	case <-done:
		return string(out), err
	case <-time.After(timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", fmt.Errorf("timeout")
	}
}

// dial connects to a listener of the test (nothing leaves this machine) and sends a harmless mark.
func dial(addr string) (bool, string) {
	if addr == "" {
		return false, "no listener"
	}
	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err == nil {
		c.Write([]byte("rivus-sbx marker\n"))
		c.Close()
	}
	return err == nil, ""
}

func main() {
	flag.Parse()
	if *child {
		syscall.Setsid()
		time.Sleep(300 * time.Second)
		return
	}
	if *id == "" || *dir == "" {
		fmt.Fprintln(os.Stderr, "probe: -id and -dir are required")
		os.Exit(2)
	}
	home = *homeFlag
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	mark = "rivus-sbx-" + *id

	// 1. the files of the host
	check("read_canary", func() (bool, string) {
		b, err := os.ReadFile(*canary)
		return err == nil && len(b) > 0, ""
	})
	// 2. names that can be seen (only whether a path exists: nothing is read)
	check("names_visible", func() (bool, string) {
		var seen []string
		for _, p := range []string{"~/.ssh", "~/.cargo", "~/.gitconfig", "~/.netrc", "~/.config/gh", "~/.aws", "/Users", "/Volumes", "/Applications", "~"} {
			q := strings.Replace(p, "~", home, 1)
			if _, err := os.Lstat(q); err == nil {
				seen = append(seen, p)
			}
		}
		listable := ""
		if es, err := os.ReadDir(home); err == nil {
			listable = fmt.Sprintf(" home_listable=%d_entries", len(es))
		}
		return len(seen) > 0 || listable != "", "visible=" + strings.Join(seen, ",") + listable
	})
	// 3. writing
	check("write_outside", func() (bool, string) {
		err := os.WriteFile(filepath.Join(*dir, mark+"-w"), nil, 0o600)
		return err == nil, ""
	})
	check("write_home", func() (bool, string) {
		err := os.WriteFile(filepath.Join(home, mark+"-h"), nil, 0o600)
		return err == nil, ""
	})
	// 4. settings that act later
	check("defaults_write", func() (bool, string) {
		_, err := run(5*time.Second, "/usr/bin/defaults", "write", mark, "marker", "x")
		return err == nil, ""
	})
	check("launchagents_file", func() (bool, string) {
		err := os.WriteFile(filepath.Join(home, "Library", "LaunchAgents", mark+".plist"), nil, 0o600)
		return err == nil, ""
	})
	check("shell_rc_file", func() (bool, string) {
		err := os.WriteFile(filepath.Join(home, "."+mark+"-rc"), nil, 0o600)
		return err == nil, ""
	})
	check("crontab_list", func() (bool, string) {
		// only whether crontab can be asked (it exits 1 with "no crontab" when it can and there is none)
		out, err := run(5*time.Second, "/usr/bin/crontab", "-l")
		return err == nil || strings.Contains(out, "no crontab"), ""
	})
	// 5. other applications and services doing work for the build
	check("osascript_finder_folder", func() (bool, string) {
		target := filepath.Join(*dir, mark+"-finder")
		_, _ = run(20*time.Second, "/usr/bin/osascript", "-e",
			fmt.Sprintf(`tell application "Finder" to make new folder at (POSIX file %q as alias) with properties {name:%q}`, *dir, mark+"-finder"))
		_, err := os.Stat(target)
		return err == nil, ""
	})
	check("launchctl_submit", func() (bool, string) {
		target := filepath.Join(*dir, mark+"-job")
		_, _ = run(5*time.Second, "/bin/launchctl", "submit", "-l", mark, "--", "/usr/bin/touch", target)
		for i := 0; i < 20; i++ {
			if _, err := os.Stat(target); err == nil {
				return true, ""
			}
			time.Sleep(200 * time.Millisecond)
		}
		return false, ""
	})
	check("open_finder", func() (bool, string) {
		_, err := run(10*time.Second, "/usr/bin/open", "-g", "-a", "Finder")
		return err == nil, ""
	})
	// 6. the pasteboard and the keychain: green only (the test does not run these without the sandbox)
	check("pbpaste", func() (bool, string) {
		cmd := exec.Command("/usr/bin/pbpaste") // the output goes nowhere: nothing of the pasteboard is read here
		return cmd.Run() == nil, ""
	})
	check("securityd_reachable", func() (bool, string) {
		// the authorization service (a mach service of the system) answers with the plist of a right: reading it is harmless,
		// and it says whether the services of security can be reached (the keychain is another of them)
		cmd := exec.Command("/usr/bin/security", "authorizationdb", "read", "system.preferences")
		return cmd.Run() == nil, ""
	})
	// 7. UNIX domain sockets
	check("unix_socket_test", func() (bool, string) {
		c, err := net.DialTimeout("unix", *sock, 2*time.Second)
		if err != nil {
			return false, ""
		}
		c.Write([]byte("rivus-sbx marker\n"))
		c.Close()
		return true, ""
	})
	check("unix_socket_docker", func() (bool, string) {
		c, err := net.DialTimeout("unix", "/var/run/docker.sock", 2*time.Second)
		if err == nil {
			c.Close()
		}
		return err == nil, ""
	})
	check("unix_socket_ssh_agent", func() (bool, string) {
		p := os.Getenv("SSH_AUTH_SOCK")
		if p == "" {
			return false, "no SSH_AUTH_SOCK in the environment"
		}
		c, err := net.DialTimeout("unix", p, 2*time.Second)
		if err == nil {
			c.Close()
		}
		return err == nil, ""
	})
	// 8. the network (the listeners of the test only: nothing goes out of this machine)
	check("network_loopback", func() (bool, string) { return dial(*tcp) })
	check("network_lan", func() (bool, string) { return dial(*lan) })
	check("network_udp", func() (bool, string) {
		c, err := net.DialTimeout("udp", *udp, 2*time.Second)
		if err != nil {
			return false, ""
		}
		_, err = c.Write([]byte("rivus-sbx marker\n"))
		c.Close()
		return err == nil, ""
	})
	check("network_ipv6_loopback", func() (bool, string) { return dial(*tcp6) })
	// 9. DNS: the path itself, whether it can be reached (nothing is sent)
	check("dns_socket", func() (bool, string) {
		c, err := net.DialTimeout("unix", "/var/run/mDNSResponder", 2*time.Second)
		if err == nil {
			c.Close()
		}
		return err == nil, ""
	})
	// 10. the terminal
	check("dev_tty", func() (bool, string) {
		f, err := os.Open("/dev/tty")
		if err == nil {
			f.Close()
		}
		return err == nil, ""
	})
	// 11. the environment
	check("env_token", func() (bool, string) {
		return os.Getenv(fakeEnv) != "", ""
	})
	// 12, 20, 21. other processes of the same user
	check("signal_other_process", func() (bool, string) {
		return *pid > 0 && syscall.Kill(*pid, 0) == nil, ""
	})
	check("read_env_of_other_process", func() (bool, string) {
		// kern.procargs2 holds the arguments and the environment of a process of the same user
		b, err := unix.SysctlRaw("kern.procargs2", *pid)
		return err == nil && *token != "" && strings.Contains(string(b), *token), ""
	})
	// the range of kern.procargs2: which processes of the machine can be asked (the answer is yes or no: what they hold is not kept)
	check("procargs_range", func() (bool, string) {
		var parts []string
		any := false
		for _, t := range strings.Split(*targets, ",") {
			kv := strings.SplitN(t, "=", 2)
			if len(kv) != 2 {
				continue
			}
			var p int
			fmt.Sscan(kv[1], &p)
			b, err := unix.SysctlRaw("kern.procargs2", p)
			ok := err == nil && len(b) > 8
			b = nil
			any = any || ok
			parts = append(parts, fmt.Sprintf("%s=%v", kv[0], ok))
		}
		return any, strings.Join(parts, " ")
	})
	// shared memory: a name of POSIX shared memory that the program makes (and removes)
	check("shm_open", func() (bool, string) {
		name, _ := syscall.BytePtrFromString("/" + mark + "-shm")
		fd, _, e := syscall.Syscall(unix.SYS_SHM_OPEN, uintptr(unsafe.Pointer(name)), uintptr(syscall.O_RDWR|syscall.O_CREAT), 0o600)
		if e != 0 {
			return false, ""
		}
		syscall.Close(int(fd))
		syscall.Syscall(unix.SYS_SHM_UNLINK, uintptr(unsafe.Pointer(name)), 0, 0)
		return true, ""
	})
	// 17. the toolchain
	check("write_toolchain", func() (bool, string) {
		if *tool == "" {
			return false, "no toolchain directory"
		}
		err := os.WriteFile(filepath.Join(*tool, mark+"-t"), nil, 0o600)
		return err == nil, ""
	})
	// 23. the preferences of the user (a domain that the test wrote outside the sandbox)
	check("defaults_read", func() (bool, string) {
		out, err := run(5*time.Second, "/usr/bin/defaults", "read", mark, "marker")
		return err == nil && strings.TrimSpace(out) == "x", ""
	})
	// 14. a process that stays after the run (setsid, double fork), marked with the id in its environment
	if *daemon {
		// a double fork: the shell starts this program again in the background and goes away; the program makes a session of its own
		self, _ := os.Executable()
		// the shell starts this program again in the background, prints its pid, and goes away: the program makes a session of its own
		cmd := exec.Command("/bin/sh", "-c", "('"+self+"' -child >/dev/null 2>&1 & echo $!)")
		out, err := cmd.Output()
		if pid := strings.TrimSpace(string(out)); err == nil && pid != "" {
			fmt.Printf("LEFT_PID %s\n", pid) // the test is told, because it looks for nothing
		}
		report("leave_process", err == nil, "")
	}
}
