//go:build darwin || linux

package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/sandbox"
)

// SBX-4: the standard error of a stage reaches the user, and the error of a failed stage, without a control character.
func TestSandboxResultStripsControlCharacters(t *testing.T) {
	const seq = "\x1b]52;c;AAAA\x07 \x1b[2J \u009b \u009d \r x\x7f"
	for _, code := range []int{0, 1} {
		var buf bytes.Buffer
		o := &isolationOpts{stderr: &buf, timeout: time.Minute}
		err := o.sandboxResult("extract", &sandbox.Result{ExitCode: code, Stderr: []byte("out: " + seq + "\n")}, nil)
		if (code != 0) != (err != nil) {
			t.Fatalf("exit %d: err = %v", code, err)
		}
		if i := strings.IndexAny(buf.String(), "\x1b\x07\r\x7f\u009b\u009d"); i >= 0 {
			t.Errorf("exit %d: a control character is on the standard error: %q", code, buf.String())
		}
		if !strings.Contains(buf.String(), `\x1b]52;c;AAAA`) {
			t.Errorf("exit %d: the text is gone: %q", code, buf.String())
		}
		if err != nil && strings.ContainsAny(err.Error(), "\x1b\x07\r\x7f\u009b\u009d") {
			t.Errorf("exit %d: a control character is in the error: %q", code, err.Error())
		}
	}
}
