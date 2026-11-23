package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// envIRSpillFault is read by the binary that is built with the tag irspillfault (irspill_fault.go), and by no other.
const envIRSpillFault = "RIVUS_INTERNAL_IR_SPILL_FAULT"

var (
	faultBinaryOnce sync.Once
	faultBinaryPath string
)

// faultBinary builds the binary with the failure put in (the tag irspillfault): the one that is shipped does not have it.
func faultBinary(t *testing.T) string {
	t.Helper()
	faultBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "rivus-fault")
		if err != nil {
			t.Fatal(err)
		}
		faultBinaryPath = filepath.Join(dir, "rivus")
		if out, err := exec.Command("go", "build", "-tags", "irspillfault", "-o", faultBinaryPath, ".").CombinedOutput(); err != nil {
			faultBinaryPath = ""
			t.Logf("%s", out)
		}
	})
	if faultBinaryPath == "" {
		t.Fatal("the binary with the failure put in was not built")
	}
	return faultBinaryPath
}

// A function of the IR that cannot be read in the file stops the run with an internal error (exit code 1) and one line
// that names the function, wherever in the run it happens: when the sites are listed, when the calls of function
// values get their candidates, when the graph is built. Nothing is skipped, and nothing ends in a panic of the runtime.
// The failure is put in by RIVUS_INTERNAL_IR_SPILL_FAULT=n, which only the binary built with the tag irspillfault reads: every read from the n-th on fails. (The reads of the IR are
// all before the analysis starts: the workers do not read it, internal/ir TestWorkersDoNotReadTheSource.)
func TestIRSpillReadFailureStopsTheRun(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"analyze", "-C", dir, "-v", "-rules", filepath.Join(dir, "rules.yaml"), "./..."}
	run := func(env ...string) (int, string) {
		cmd := exec.Command(faultBinary(t), args...)
		cmd.Env = append(cmd.Environ(), env...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return code, stderr.String()
	}
	code, out := run()
	m := regexp.MustCompile(`(\d+) reads`).FindStringSubmatch(out)
	if code != 0 || m == nil {
		t.Fatalf("the run without a failure: exit %d\n%s", code, out)
	}
	reads, _ := strconv.Atoi(m[1])
	// the first read, one in the middle, the last (in the build of the graph)
	for _, n := range []int{1, reads / 3, reads / 2, reads} {
		code, out := run(envIRSpillFault + "=" + strconv.Itoa(n))
		if code != 1 {
			t.Errorf("the read %d of %d fails: exit %d, want 1", n, reads, code)
		}
		var lines []string
		for _, l := range strings.Split(out, "\n") {
			if strings.Contains(l, "cannot be read") {
				lines = append(lines, l)
			}
		}
		if len(lines) != 1 || !strings.HasPrefix(lines[0], "rivus: internal error: the IR of ") || !strings.Contains(lines[0], envIRSpillFault) {
			t.Errorf("the read %d of %d fails: the message is %q", n, reads, lines)
		}
		if strings.Contains(out, "goroutine ") || strings.Contains(out, "panic:") {
			t.Errorf("the read %d of %d fails: the run ended in a panic:\n%s", n, reads, out)
		}
	}
}

// The IR goes to a file in the temporary directory by default. When that directory cannot be written, the run goes on
// with the IR in memory, says it on one line, and has the result of the run with the file. (The run reads an IR stream:
// the go command that reads Go code needs the temporary directory itself.)
func TestIRSpillTMPDIRNotWritable(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	irFile := filepath.Join(t.TempDir(), "ir.jsonl")
	if out, err := exec.Command(binary, "debug", "extract", "-C", dir, "./...").Output(); err != nil {
		t.Fatal(err)
	} else if err := os.WriteFile(irFile, out, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"analyze", "-C", dir, "-ir", irFile, "-rules", filepath.Join(dir, "rules.yaml")}
	run := func(tmp string) (int, string, string) {
		cmd := exec.Command(binary, args...)
		cmd.Env = append(cmd.Environ(), "TMPDIR="+tmp)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		code := 0
		var ee *exec.ExitError
		if err := cmd.Run(); errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return code, stdout.String(), stderr.String()
	}
	wantCode, wantOut, errWith := run(t.TempDir())
	if strings.Contains(errWith, "held in memory") || len(wantOut) < 100 {
		t.Fatalf("the run with the file: exit %d\n%s", wantCode, errWith)
	}
	code, out, errOut := run(filepath.Join(t.TempDir(), "no", "such", "dir"))
	if code != wantCode || code > 4 || findingsOf(t, out) != findingsOf(t, wantOut) {
		t.Errorf("TMPDIR that cannot be written: exit %d (want %d), the findings are the same: %v\n%s", code, wantCode, findingsOf(t, out) == findingsOf(t, wantOut), errOut)
	}
	var warn []string
	for _, l := range strings.Split(errOut, "\n") {
		if strings.Contains(l, "held in memory") {
			warn = append(warn, l)
		}
	}
	if len(warn) != 1 || strings.Contains(errOut, "goroutine ") || strings.Contains(errOut, "panic:") {
		t.Errorf("want one warning line and no panic: %q\n%s", warn, errOut)
	}
}

// findingsOf is the findings of a report as text (the times and the memory of a run are not the same twice).
func findingsOf(t *testing.T, report string) string {
	t.Helper()
	var r struct {
		Findings json.RawMessage `json:"findings"`
	}
	if err := json.Unmarshal([]byte(report), &r); err != nil || len(r.Findings) < 3 {
		t.Fatalf("the report has no findings: %v", err)
	}
	return string(r.Findings)
}
