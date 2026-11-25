//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The calls that change attributes (mode, owner, times, extended attributes) are done by rivus for the program, and only for the files inside of the places that may
// be written; for a file outside the answer is EPERM and it is reported. The program cannot make rivus do one thing and say another: the path is read once, and the
// file that is checked is the file that is opened and changed (notify_linux.go).

func attrRun(t *testing.T, f fixture, read []string, args ...string) *Result {
	t.Helper()
	res, err := Run(context.Background(), Spec{Argv: append([]string{f.probe}, args...), Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"}, Read: read, Write: []string{f.work}, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s %s", res.ExitCode, res.Stdout, res.Stderr)
	}
	return res
}

func modeOf(t *testing.T, p string) os.FileMode {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.Mode().Perm()
}

func realPath(p string) string {
	r, _ := filepath.EvalSymlinks(p)
	return r
}

// A change of a file inside is done; the one of a file outside is refused (EPERM), the file is not touched, and the refusal is reported.
func TestSandboxAttrInsideIsDoneOutsideIsRefused(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	victim := filepath.Join(f.outside, "victim")
	os.WriteFile(victim, []byte("data"), 0o600)
	mine := filepath.Join(f.work, "mine")
	os.WriteFile(mine, []byte("data"), 0o600)
	before, _ := os.Stat(victim)
	res := attrRun(t, f, nil, "-attr", victim, "-attr-in", mine)
	if got := modeOf(t, victim); got != 0o600 {
		t.Errorf("the mode of the file outside was changed: %v", got)
	}
	if after, _ := os.Stat(victim); !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("the times of the file outside were changed")
	}
	if got := modeOf(t, mine); got != 0o640 {
		t.Errorf("the mode of the file inside was not changed: %v", got)
	}
	if !strings.Contains(string(res.Stdout), "attr-chmod chmod "+victim+": operation not permitted") {
		t.Errorf("the program was not answered EPERM:\n%s", res.Stdout)
	}
	if !strings.Contains(string(res.Stdout), "attr-in-chmod <nil>") {
		t.Errorf("the change inside failed:\n%s", res.Stdout)
	}
	got := strings.Join(res.AttributeChanges, "\n")
	if !strings.Contains(got, realPath(victim)) {
		t.Errorf("the refusal is not reported:\n%s", got)
	}
	if strings.Contains(got, "mine") {
		t.Errorf("a change inside is reported:\n%s", got)
	}
}

// `..` after a link is the parent of what the link points to, as the kernel reads it: work/link/../victim is the file outside, and it is refused.
func TestSandboxAttrDotDotAfterALink(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	sub := filepath.Join(f.outside, "sub")
	os.Mkdir(sub, 0o755)
	victim := filepath.Join(f.outside, "victim")
	os.WriteFile(victim, []byte("data"), 0o600)
	decoy := filepath.Join(f.work, "victim") // what a cleaned string would name: a file inside
	os.WriteFile(decoy, []byte("data"), 0o600)
	if err := os.Symlink(sub, filepath.Join(f.work, "link")); err != nil {
		t.Fatal(err)
	}
	res := attrRun(t, f, nil, "-attr-dotdot", f.work+"/link/../victim")
	if got := modeOf(t, victim); got != 0o600 {
		t.Errorf("the file outside was changed through a link and a dotdot: %v", got)
	}
	if got := modeOf(t, decoy); got != 0o600 {
		t.Errorf("the decoy was changed: %v", got)
	}
	if got := strings.Join(res.AttributeChanges, "\n"); !strings.HasSuffix(got, realPath(victim)) {
		t.Errorf("the refusal of %s is not reported:\n%s", realPath(victim), got)
	}
}

// A call about a link itself (lchown) is about the link, which is inside: it is done, and what the link points to (outside) is not touched. A call with an empty path
// and AT_EMPTY_PATH is about the file of the descriptor: outside, so refused.
func TestSandboxAttrLinkItselfAndEmptyPath(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	victim := filepath.Join(f.outside, "victim")
	os.WriteFile(victim, []byte("data"), 0o600)
	link := filepath.Join(f.work, "link")
	if err := os.Symlink(victim, link); err != nil {
		t.Fatal(err)
	}
	readable := filepath.Join(f.outside, "readable")
	os.WriteFile(readable, []byte("data"), 0o600)
	res := attrRun(t, f, []string{f.outside}, "-attr-link", link, "-attr-fd", readable)
	if !strings.Contains(string(res.Stdout), "attr-link-lchown <nil>") {
		t.Errorf("lchown of a link inside failed:\n%s", res.Stdout)
	}
	got := strings.Join(res.AttributeChanges, "\n")
	if strings.Contains(got, realPath(victim)) {
		t.Errorf("lchown of a link inside is taken for a change of what it points to:\n%s", got)
	}
	if !strings.Contains(got, "fchownat "+realPath(readable)) || !strings.Contains(string(res.Stdout), "attr-fd-fchownat operation not permitted") {
		t.Errorf("fchownat with AT_EMPTY_PATH on a file outside is not refused and reported:\n%s\n%s", got, res.Stdout)
	}
}

// Every form that is done for the program gives what it gives outside of the sandbox: the same answers and the same files (the sandbox does not change what a
// tool sees, for the files that are inside).
func TestSandboxAttrFormsAreDoneAsOnTheHost(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	hostDir := t.TempDir()
	host, err := exec.Command(f.probe, "-attr-all", hostDir).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, host)
	}
	res := attrRun(t, f, nil, "-attr-all", f.work)
	// (the probe says what the other checks found after these lines, and those are not the same inside and outside: only the lines of the calls are compared)
	upTo := func(b []byte) string {
		s := string(b)
		if i := strings.Index(s, "final:"); i >= 0 {
			if j := strings.Index(s[i:], "\n"); j >= 0 {
				return s[:i+j+1]
			}
		}
		return s
	}
	if h, g := upTo(host), upTo(res.Stdout); h != g {
		t.Errorf("the answers differ from the ones of the host.\nhost:\n%s\nsandbox:\n%s\n(refused: %v)", h, g, res.AttributeChanges)
	}
	if len(res.AttributeChanges) != 0 || len(res.Warnings) > 1 { // (one warning may be there: an old kernel)
		t.Errorf("something was refused or not checked: %v %v", res.AttributeChanges, res.Warnings)
	}
	if !strings.Contains(string(host), "final: mode=") {
		t.Errorf("the probe did not get to the end:\n%s", host)
	}
}

// A path through /proc/self is the program's /proc/self, not rivus's: chmod on /proc/self/fd/N is the file of the descriptor of the program.
func TestSandboxAttrProcSelfIsTheProgram(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	mine := filepath.Join(f.work, "mine")
	os.WriteFile(mine, []byte("data"), 0o600)
	theirs := filepath.Join(f.outside, "theirs")
	os.WriteFile(theirs, []byte("data"), 0o600)
	res := attrRun(t, f, []string{f.outside}, "-attr-proc-in", mine, "-attr-proc-out", theirs)
	if got := modeOf(t, mine); got != 0o640 {
		t.Errorf("chmod through /proc/self/fd/N of a file inside did not work: %v\n%s", got, res.Stdout)
	}
	if got := modeOf(t, theirs); got != 0o600 {
		t.Errorf("chmod through /proc/self/fd/N of a file outside was done: %v", got)
	}
	if !strings.Contains(string(res.Stdout), "proc-out-chmod: operation not permitted") {
		t.Errorf("the program was not answered EPERM for the file outside:\n%s", res.Stdout)
	}
	if !strings.Contains(strings.Join(res.AttributeChanges, "\n"), realPath(theirs)) {
		t.Errorf("the refusal of the file outside is not reported: %v", res.AttributeChanges)
	}
}

// The attack on the check of a path: another thread changes the path (one byte, A/ to B/) after the call is looked at. The file that is changed is the one that was
// checked: no file of B/ (outside) is ever changed.
func TestSandboxAttrRaceOnAPath(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	root, _ := filepath.EvalSymlinks(t.TempDir())
	a, b := filepath.Join(root, "A"), filepath.Join(root, "B")
	os.Mkdir(a, 0o755)
	os.Mkdir(b, 0o755)
	const n = 400
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("f%03d", i)
		os.WriteFile(filepath.Join(a, name), nil, 0o600)
		os.WriteFile(filepath.Join(b, name), nil, 0o600)
	}
	res, err := Run(context.Background(), Spec{Argv: []string{f.probe, "-race-path", root}, Dir: a, Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{a}, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	outside, inside := 0, 0
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("f%03d", i)
		if modeOf(t, filepath.Join(b, name)) != 0o600 {
			outside++
		}
		if modeOf(t, filepath.Join(a, name)) == 0o640 {
			inside++
		}
	}
	if outside != 0 {
		t.Errorf("%d files outside were changed by a call that was checked as a call on a file inside", outside)
	}
	if inside == 0 {
		t.Errorf("no call on a file inside was done (the test does not test what it says): %v", res.AttributeChanges)
	}
	t.Logf("%d of %d calls on a file inside were done; %d refused as outside", inside, n, len(res.AttributeChanges))
}

// The attack on the check of a descriptor: another thread makes the descriptor refer to the file outside (dup3) after the call is looked at.
func TestSandboxAttrRaceOnADescriptor(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	inside := filepath.Join(f.work, "inside")
	os.WriteFile(inside, nil, 0o600)
	outside := filepath.Join(f.outside, "outside")
	os.WriteFile(outside, nil, 0o600)
	attrRun(t, f, []string{f.outside}, "-race-fd", inside+":"+outside)
	if got := modeOf(t, outside); got != 0o600 {
		t.Errorf("the file outside was changed through a descriptor that another thread changed: %v", got)
	}
	if got := modeOf(t, inside); got != 0o640 {
		t.Errorf("no call on the file inside was done (the test does not test what it says): %v", got)
	}
}

// A call on a descriptor that is not a file (a pipe) is refused, and is not a refusal of a file of the system: nothing is reported for it.
func TestSandboxAttrOnAPipeIsRefusedInSilence(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	res := attrRun(t, f, nil, "-attr-pipe")
	if !strings.Contains(string(res.Stdout), "pipe-fchmod: operation not permitted") {
		t.Errorf("not refused: %s", res.Stdout)
	}
	if len(res.AttributeChanges) != 0 || len(res.Warnings) > 1 {
		t.Errorf("reported: %v %v", res.AttributeChanges, res.Warnings)
	}
}

// What a program that has no capability cannot do, the one that does the call for it cannot either (rivus run as root: the thread gives up its capabilities first).
func TestSandboxAttrAsRoot(t *testing.T) {
	skipWithoutSandbox(t)
	if os.Geteuid() != 0 {
		t.Skip("only for rivus run as root")
	}
	f := setup(t)
	mine := filepath.Join(f.work, "mine")
	os.WriteFile(mine, []byte("data"), 0o600)
	res := attrRun(t, f, nil, "-attr-root", mine)
	if !strings.Contains(string(res.Stdout), "chown-other: operation not permitted") {
		t.Errorf("the file was given to another user, or the answer is not EPERM:\n%s", res.Stdout)
	}
	if st, _ := os.Stat(mine); st.Sys().(*syscall.Stat_t).Uid != 0 {
		t.Errorf("the owner of the file was changed")
	}
}

// A call whose arguments cannot be read (here an address that is not mapped) is refused, and counted: the warning says that they could not be checked.
func TestSandboxAttrArgumentsThatCannotBeReadAreRefused(t *testing.T) {
	if len(attrCalls) == 0 {
		t.Skip("no calls on this architecture")
	}
	w := newAttrWatcher([]string{"/nowhere"})
	var req notifReq
	req.pid = uint32(os.Getpid())
	req.data.arch = seccompArch()
	call := attrCalls[0]
	req.data.nr = int32(call.nr)
	for i := range req.data.args {
		req.data.args[i] = 1 // an address that is not mapped
	}
	resp, send := w.handle(-1, &req)
	if !send || resp.errno != -int32(unix.EPERM) {
		t.Errorf("the call was not refused with EPERM: %+v %v", resp, send)
	}
	if got := w.changes(); len(got) != 0 {
		t.Errorf("a refusal of a file is reported for a call that could not be read: %v", got)
	}
	got := strings.Join(w.warnings(), "\n")
	if !strings.Contains(got, "refused") || !strings.Contains(got, "1 calls") {
		t.Errorf("not said as a warning: %q", got)
	}
}

// The tables of the calls: nothing is in them twice, and a call that is done for the program is not also refused.
func TestSandboxAttrTables(t *testing.T) {
	seen := map[uint32]string{}
	for _, c := range attrCalls {
		if prev, ok := seen[c.nr]; ok {
			t.Errorf("%s and %s have the same number", prev, c.name)
		}
		seen[c.nr] = c.name
	}
	for _, nr := range attrDenied {
		if name, ok := seen[nr]; ok {
			t.Errorf("%s is done for the program and is refused", name)
		}
	}
	want := []string{"fchmod", "fchmodat", "fchmodat2", "fchown", "fchownat", "utimensat", "setxattr", "lsetxattr", "fsetxattr", "removexattr", "lremovexattr", "fremovexattr"}
	have := map[string]bool{}
	for _, c := range attrCalls {
		have[c.name] = true
	}
	for _, n := range want {
		if len(attrCalls) > 0 && !have[n] {
			t.Errorf("%s is not among the calls that are done for the program", n)
		}
	}
}
