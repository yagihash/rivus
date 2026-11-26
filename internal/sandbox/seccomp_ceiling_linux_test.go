//go:build linux && (amd64 || arm64)

package sandbox

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// runFilter runs the program of the filter (classic BPF: the instructions that filter() makes) on a system call, and gives the value that it returns. It is the
// kernel's way of reading it, for the few instructions that are used; an instruction that it does not know fails the test.
func runFilter(t *testing.T, prog []unix.SockFilter, arch, nr uint32, args [6]uint32) uint32 {
	t.Helper()
	data := map[uint32]uint32{offNr: nr, offArch: arch}
	for i, a := range args {
		data[offArg0+uint32(8*i)] = a
	}
	var acc uint32
	for pc := 0; pc < len(prog); pc++ {
		in := prog[pc]
		switch in.Code {
		case bpfLdAbs:
			acc = data[in.K]
		case bpfAnd:
			acc &= in.K
		case bpfRet:
			return in.K
		case bpfJeq, bpfJge, bpfJgt:
			var ok bool
			switch in.Code {
			case bpfJeq:
				ok = acc == in.K
			case bpfJge:
				ok = acc >= in.K
			case bpfJgt:
				ok = acc > in.K
			}
			if ok {
				pc += int(in.Jt)
			} else {
				pc += int(in.Jf)
			}
		default:
			t.Fatalf("instruction %d: code %#x is not one that the test knows", pc, in.Code)
		}
	}
	t.Fatal("the program ended without a verdict")
	return 0
}

// Without the rule of the ceiling a call above it is not in the list and ends in the "allow" at the end of the filter: this test fails then (the verdict is
// SECCOMP_RET_ALLOW and not ENOSYS), for every network.
func TestSandboxSeccompCallsAboveTheCeilingAreNotThere(t *testing.T) {
	enosys := uint32(unix.SECCOMP_RET_ERRNO | uint32(unix.ENOSYS))
	for _, n := range []struct {
		name   string
		net    int
		notify bool
	}{{"none", netNone, false}, {"proxy", netProxy, false}, {"open", netOpen, false}, {"none-notify", netNone, true}} {
		t.Run(n.name, func(t *testing.T) {
			prog, err := filter(n.net, n.notify)
			if err != nil {
				t.Fatal(err)
			}
			ceiling := seccompMaxNr()
			for _, nr := range []uint32{ceiling + 1, ceiling + 2, ceiling + 1000, 0x3fffffff} {
				if got := runFilter(t, prog, seccompArch(), nr, [6]uint32{}); got != enosys {
					t.Errorf("system call %d (above the ceiling %d): verdict %#x, want ENOSYS %#x", nr, ceiling, got, enosys)
				}
			}
			// the ceiling itself and what is below are in the lists, not told to be missing
			if got := runFilter(t, prog, seccompArch(), ceiling, [6]uint32{}); got == enosys {
				t.Errorf("the ceiling %d itself is told to be missing", ceiling)
			}
			if got := runFilter(t, prog, seccompArch(), uint32(unix.SYS_IO_URING_SETUP), [6]uint32{}); got != retEPERM {
				t.Errorf("io_uring_setup: verdict %#x, want EPERM", got)
			}
			if got := runFilter(t, prog, seccompArch(), uint32(unix.SYS_READ), [6]uint32{}); got != unix.SECCOMP_RET_ALLOW {
				t.Errorf("read: verdict %#x, want ALLOW", got)
			}
		})
	}
}

// The ceiling is the highest number that golang.org/x/sys/unix knows. When x/sys is updated and knows a newer call, the filter does not: this test fails, so that
// the new calls are read (do they open a way that the filter closes in the others? then they go in deniedCalls) before the ceiling is raised.
func TestSandboxSeccompCeilingCoversXSys(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", "golang.org/x/sys/unix").Output()
	if err != nil {
		t.Skipf("no go to find golang.org/x/sys/unix: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	for _, a := range []struct {
		arch    string
		ceiling uint32
	}{{"amd64", seccompMaxNrAmd64}, {"arm64", seccompMaxNrArm64}} {
		file := filepath.Join(dir, "zsysnum_linux_"+a.arch+".go")
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		var max uint64
		var maxName string
		n := 0
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, sp := range g.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, id := range vs.Names {
					if !strings.HasPrefix(id.Name, "SYS_") || i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok {
						continue
					}
					v, err := strconv.ParseUint(lit.Value, 0, 64)
					if err != nil {
						t.Fatal(err)
					}
					n++
					if v >= max {
						max, maxName = v, id.Name
					}
				}
			}
		}
		if n < 300 {
			t.Fatalf("%s: only %d numbers of system calls were read from %s", a.arch, n, file)
		}
		if max > uint64(a.ceiling) {
			t.Errorf("%s: golang.org/x/sys/unix knows %s = %d, above the ceiling %d of the filter: read the new system calls, put the ones that need it in deniedCalls, and raise seccompMaxNr%s", a.arch, maxName, max, a.ceiling, strings.ToUpper(a.arch[:1])+a.arch[1:])
		}
		if max < uint64(a.ceiling) {
			t.Logf("%s: the ceiling %d is above the highest number in x/sys (%d, %s)", a.arch, a.ceiling, max, maxName)
		}
	}
}

// Under the sandbox, a call above the ceiling is answered with ENOSYS (the probe cannot tell it from a kernel that has no such call: the check is that the
// sandbox answers, and the test above shows that it is the filter that does).
func TestSandboxCallAboveTheCeilingIsNotThere(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	res, err := Run(context.Background(), Spec{Argv: []string{f.probe, "-only", "above-ceiling", "-ceiling", fmt.Sprint(seccompMaxNr())}, Dir: f.work,
		Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	v := verdicts(t, string(res.Stdout))
	if esc, ok := v["above-ceiling"]; !ok || esc {
		t.Fatalf("above-ceiling: %v %v\n%s\n%s", ok, esc, res.Stdout, res.Stderr)
	}
}
