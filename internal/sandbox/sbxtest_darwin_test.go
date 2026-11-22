//go:build darwin

package sandbox

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The helpers of the tests of the sandbox (docs/DESIGN-macos-sandbox.md, 3.9). Every name that these tests make starts with
// "rivus-sbx-", and the cleanup removes only those names (no wildcard that reaches further). The cleanup runs before a
// test (what an earlier run that died left) and after it.

const sbxPrefix = "rivus-sbx-"

// probeBinary builds the probe once and returns its path.
func probeBinary(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "probe")
	cmd := exec.Command("go", "build", "-o", out, "./probe")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the probe: %v\n%s", err, b)
	}
	return out
}

// sweep removes what a run of these tests (this one, or one that died) may have left: only names that start with rivus-sbx-.
func sweepNames(t *testing.T) {
	t.Helper()
	home, _ := os.UserHomeDir()
	rm := func(glob string) {
		ms, _ := filepath.Glob(glob)
		for _, m := range ms {
			if strings.Contains(filepath.Base(m), sbxPrefix) {
				os.RemoveAll(m)
			}
		}
	}
	rm(filepath.Join(home, "Library", "LaunchAgents", sbxPrefix+"*"))
	rm(filepath.Join(home, "."+sbxPrefix+"*-rc"))
	rm(filepath.Join(home, sbxPrefix+"*"))
	rm(filepath.Join("/private/tmp", sbxPrefix+"*"))
	// preferences: the domains rivus-sbx-*
	ms, _ := filepath.Glob(filepath.Join(home, "Library", "Preferences", sbxPrefix+"*.plist"))
	for _, m := range ms {
		domain := strings.TrimSuffix(filepath.Base(m), ".plist")
		if strings.HasPrefix(domain, sbxPrefix) {
			exec.Command("/usr/bin/defaults", "delete", domain).Run()
			os.Remove(m) // the file stays, empty, after the keys are deleted
		}
	}
	// launchctl jobs with the prefix
	if out, err := exec.Command("/bin/launchctl", "list").Output(); err == nil {
		for _, l := range strings.Split(string(out), "\n") {
			f := strings.Fields(l)
			if len(f) == 3 && strings.HasPrefix(f[2], sbxPrefix) {
				exec.Command("/bin/launchctl", "remove", f[2]).Run()
			}
		}
	}
	// the processes that an earlier run of these tests left, which it wrote down (left.pid) in the directory of that run
	ms, _ = filepath.Glob(filepath.Join("/private/tmp", sbxPrefix+"*-dir", "left.pid"))
	for _, m := range ms {
		if b, err := os.ReadFile(m); err == nil {
			for _, l := range strings.Split(string(b), "\n") {
				if kv := strings.SplitN(l, "|", 2); len(kv) == 2 {
					if n, err := strconv.Atoi(kv[0]); err == nil {
						killKnown(n, kv[1])
					}
				}
			}
		}
	}
}

// startOf is the time that a process started, as ps says it ("" when there is no such process).
func startOf(pid int) string {
	out, err := exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// killKnown kills a process that this test knows by its pid (the probe says it: LEFT_PID) and by the time it started (taken
// when the pid was reported), after it has checked, with ps, that the process is still the probe in its sleeping mode. A pid
// is used again: the start time is what says that it is the same process. Nothing is looked for: a pid that is not known is
// not touched, and the signal goes to that one process only.
func killKnown(pid int, started string) bool {
	if pid <= 1 || started == "" || startOf(pid) != started {
		return false
	}
	out, err := exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil || !strings.Contains(string(out), "probe") || !strings.Contains(string(out), "-child") {
		return false
	}
	return syscall.Kill(pid, syscall.SIGKILL) == nil
}

// leftPIDs reads the pids that the probe reported (LEFT_PID n) in the output of a run.
func leftPIDs(out []byte) []int {
	var pids []int
	for _, l := range strings.Split(string(out), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[0] == "LEFT_PID" {
			if n, err := strconv.Atoi(f[1]); err == nil {
				pids = append(pids, n)
			}
		}
	}
	return pids
}

// lanAddress is an IPv4 address of this machine that is not the loopback (the address a listener on the LAN would have).
func lanAddress() string {
	as, _ := net.InterfaceAddrs()
	for _, a := range as {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() && !ipn.IP.IsLinkLocalUnicast() {
			return ipn.IP.String()
		}
	}
	return ""
}

// world is what a test of the probe stands up: the listeners, a fake secret, the process with a fake token. close removes it.
type world struct {
	id, dir, canary, tcp, tcp6, lan, sock, udp, token string
	pid                                               int
	closers                                           []func()
}

func (w *world) close() {
	for i := len(w.closers) - 1; i >= 0; i-- {
		w.closers[i]()
	}
}

func newWorld(t *testing.T, probe string) *world {
	t.Helper()
	sweepNames(t)
	id := fmt.Sprintf("%x", rand.New(rand.NewSource(time.Now().UnixNano())).Int63())
	w := &world{id: id, token: "tok-" + id}
	// a directory of the test that the probe may not write to, and the fake secret
	w.dir = filepath.Join("/private/tmp", sbxPrefix+id+"-dir")
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	w.closers = append(w.closers, func() { os.RemoveAll(w.dir) })
	w.canary = filepath.Join(w.dir, "canary.txt")
	os.WriteFile(w.canary, []byte("a fake secret "+id+"\n"), 0o644)
	// listeners (a random port; only while the test runs)
	listen := func(network, addr string) (net.Listener, string) {
		l, err := net.Listen(network, addr)
		if err != nil {
			t.Fatalf("listen %s %s: %v", network, addr, err)
		}
		go func() {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				c.Close()
			}
		}()
		w.closers = append(w.closers, func() { l.Close() })
		return l, l.Addr().String()
	}
	_, w.tcp = listen("tcp", "127.0.0.1:0")
	// the same port on the IPv6 loopback, when the machine has one (a listener of its own: another port)
	if l6, err := net.Listen("tcp", "[::1]:0"); err == nil {
		go func() {
			for {
				c, err := l6.Accept()
				if err != nil {
					return
				}
				c.Close()
			}
		}()
		w.tcp6 = l6.Addr().String()
		w.closers = append(w.closers, func() { l6.Close() })
	}
	if ip := lanAddress(); ip != "" {
		_, w.lan = listen("tcp", ip+":0")
	}
	_, w.sock = listen("unix", filepath.Join("/private/tmp", sbxPrefix+id+".sock"))
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	w.udp = pc.LocalAddr().String()
	w.closers = append(w.closers, func() { pc.Close() })
	// a process of the same user that holds the fake token in its environment
	// (the probe itself in its sleeping mode: a program that is not Apple's, whose environment other processes may read)
	sleep := exec.Command(probe, "-child")
	sleep.Env = []string{"RIVUS_SBX_MARK=" + sbxPrefix + id, "FAKE_TOKEN=" + w.token}
	if err := sleep.Start(); err != nil {
		t.Fatal(err)
	}
	w.pid = sleep.Process.Pid
	w.closers = append(w.closers, func() { sleep.Process.Kill(); sleep.Wait() })
	// a preference domain written outside the sandbox
	exec.Command("/usr/bin/defaults", "write", sbxPrefix+id, "marker", "x").Run()
	w.closers = append(w.closers, func() { exec.Command("/usr/bin/defaults", "delete", sbxPrefix+id).Run() })
	// the sweep after the test, whatever the probe left
	w.closers = append(w.closers, func() { sweepNames(t) })
	return w
}

// args are the flags of the probe for this world.
func (w *world) args(extra ...string) []string {
	a := []string{"-tcp6", w.tcp6, "-id", w.id, "-dir", w.dir, "-canary", w.canary, "-tcp", w.tcp, "-lan", w.lan, "-sock", w.sock, "-udp", w.udp,
		"-pid", strconv.Itoa(w.pid), "-token", w.token}
	return append(a, extra...)
}

// noteLeft writes down the pids that the probe reported as left behind (so that a later run can remove them even if this one dies),
// and removes them when the test ends. It returns them.
func (w *world) noteLeft(out []byte) []int {
	pids := leftPIDs(out)
	starts := map[int]string{}
	var f []string
	for _, p := range pids {
		starts[p] = startOf(p) // taken now, at once: a pid that is used again later has another start time
		f = append(f, strconv.Itoa(p)+"|"+starts[p])
	}
	os.WriteFile(filepath.Join(w.dir, "left.pid"), []byte(strings.Join(f, "\n")), 0o644)
	w.closers = append(w.closers, func() {
		for _, p := range pids {
			killKnown(p, starts[p])
		}
	})
	return pids
}
