package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A function of the IR that cannot be read in the file stops the run with an internal error (exit code 1) and one line
// that names the function, wherever in the run it happens: when the sites are listed, when the calls of function
// values get their candidates, when the graph is built. Nothing is skipped, and nothing ends in a panic of the runtime.
// The failure is put in by RIVUS_INTERNAL_IR_SPILL_FAULT=n: every read from the n-th on fails. (The reads of the IR are
// all before the analysis starts: the workers do not read it, internal/ir TestWorkersDoNotReadTheSource.)
func TestIRSpillReadFailureStopsTheRun(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"analyze", "-C", dir, "-v", "-rules", filepath.Join(dir, "rules.yaml"), "./..."}
	run := func(env ...string) (int, string) {
		cmd := exec.Command(binary, args...)
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
	code, out := run(envIRSpill + "=1")
	m := regexp.MustCompile(`(\d+) reads`).FindStringSubmatch(out)
	if code != 0 || m == nil {
		t.Fatalf("the run without a failure: exit %d\n%s", code, out)
	}
	reads, _ := strconv.Atoi(m[1])
	// the first read, one in the middle, the last (in the build of the graph)
	for _, n := range []int{1, reads / 3, reads / 2, reads} {
		code, out := run(envIRSpill+"=1", envIRSpillFault+"="+strconv.Itoa(n))
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
