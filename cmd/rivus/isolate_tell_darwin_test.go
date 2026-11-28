//go:build darwin

package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/sandbox"
)

// What the run did that the user must hear is said when the stage failed or was stopped too: a build script that starts a process in a
// session of its own and then fails (or runs out of time) is the one to look at. Before, the notice came after the returns of the failures.
func TestSandboxResultTellsEvenWhenTheStageFailed(t *testing.T) {
	for name, res := range map[string]*sandbox.Result{
		"exit":      {ExitCode: 1},
		"timeout":   {TimedOut: true},
		"cancelled": {Cancelled: true},
		"toobig":    {OutputTooBig: true},
		"success":   {},
	} {
		res.Warnings = []string{"a warning of the run"}
		res.MayHaveLeftProcesses = true
		var buf bytes.Buffer
		o := &isolationOpts{stderr: &buf, timeout: time.Minute}
		err := o.sandboxResult("extract", res, nil)
		if (name == "success") != (err == nil) {
			t.Errorf("%s: err = %v", name, err)
		}
		for _, want := range []string{"a warning of the run", "left a process behind"} {
			if !strings.Contains(buf.String(), want) {
				t.Errorf("%s: %q is not said:\n%s", name, want, buf.String())
			}
		}
	}
	// the fetch stage runs no code of the project: nothing of this is said for it
	var buf bytes.Buffer
	o := &isolationOpts{stderr: &buf}
	if err := o.sandboxResult("fetch", &sandbox.Result{MayHaveLeftProcesses: true}, nil); err != nil || buf.Len() != 0 {
		t.Errorf("fetch: %v %q", err, buf.String())
	}
}
