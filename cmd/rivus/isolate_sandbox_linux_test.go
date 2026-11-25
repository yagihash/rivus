//go:build linux

package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/sandbox"
)

// What the run did that the user must hear (a warning, a change of the attributes of a file outside, a process that may have been left) is said when the
// stage failed or was stopped too: that is when a build script that did it and then failed (or ran out of time) is the one that the user should look at.
func TestSandboxResultTellsEvenWhenTheStageFailed(t *testing.T) {
	for name, res := range map[string]*sandbox.Result{
		"exit":    {ExitCode: 1},
		"timeout": {TimedOut: true},
		"toobig":  {OutputTooBig: true},
		"success": {},
	} {
		res.Warnings = []string{"this kernel has Landlock ABI 5"}
		res.AttributeChanges = []string{"chmod /home/u/.ssh/id_rsa"}
		res.MayHaveLeftProcesses = true
		var buf bytes.Buffer
		o := &isolationOpts{stderr: &buf, timeout: time.Minute}
		err := o.sandboxResult("extract", res, nil)
		if (name == "success") != (err == nil) {
			t.Errorf("%s: err = %v", name, err)
		}
		for _, want := range []string{"this kernel has Landlock ABI 5", "chmod /home/u/.ssh/id_rsa", "left a process behind"} {
			if !strings.Contains(buf.String(), want) {
				t.Errorf("%s: %q is not said:\n%s", name, want, buf.String())
			}
		}
	}
	// the fetch stage runs no code of the project: nothing of this is said for it
	var buf bytes.Buffer
	o := &isolationOpts{stderr: &buf}
	if err := o.sandboxResult("fetch", &sandbox.Result{AttributeChanges: []string{"chmod /x"}, MayHaveLeftProcesses: true}, nil); err != nil || strings.Contains(buf.String(), "chmod") {
		t.Errorf("fetch: %v %q", err, buf.String())
	}
}
