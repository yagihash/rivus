package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const zero = "0000000000000000000000000000000000000000"

// runHook runs .githooks/pre-push with a fake `gh` (a shell script that prints ghOut and exits with ghCode) first on PATH.
// ghOut == "" with ghCode == -1 means no gh at all.
func runHook(t *testing.T, remote, stdin, ghOut string, ghCode int, env ...string) (string, error) {
	t.Helper()
	bin := t.TempDir()
	// the hook needs bash, awk and the like: link the system tools it uses, but not gh
	for _, tool := range []string{"bash", "awk", "cat", "printf", "env"} {
		if p, err := exec.LookPath(tool); err == nil {
			os.Symlink(p, filepath.Join(bin, tool))
		}
	}
	if ghCode >= 0 {
		script := "#!/bin/sh\ncat <<'EOF'\n" + ghOut + "\nEOF\nexit " + string(rune('0'+ghCode)) + "\n"
		if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	hook, _ := filepath.Abs("../.githooks/pre-push")
	cmd := exec.Command(hook, remote, "url")
	cmd.Env = append([]string{"PATH=" + bin}, env...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestPrePushHook(t *testing.T) {
	push := "refs/heads/x 1111 refs/heads/feat 2222\n"
	cases := []struct {
		name   string
		remote string
		stdin  string
		gh     string
		code   int
		env    []string
		refuse bool
		want   string
	}{
		{"merged only", "origin", push, "MERGED 12", 0, nil, true, "#12"},
		{"closed only", "origin", push, "CLOSED 13", 0, nil, true, "#13"},
		{"open and merged", "origin", push, "MERGED 12\nOPEN 14", 0, nil, false, ""},
		{"none", "origin", push, "", 0, nil, false, ""},
		{"gh missing", "origin", push, "", -1, nil, false, "WARNING"},
		{"gh fails", "origin", push, "", 1, nil, false, "WARNING"},
		{"delete", "origin", "(delete) " + zero + " refs/heads/feat 2222\n", "MERGED 12", 0, nil, false, ""},
		{"other remote", "fork", push, "MERGED 12", 0, nil, false, ""},
		{"tag", "origin", "refs/tags/v1 1111 refs/tags/v1 " + zero + "\n", "MERGED 12", 0, nil, false, ""},
		{"override", "origin", push, "MERGED 12", 0, []string{"RIVUS_ALLOW_PUSH_TO_MERGED=1"}, false, ""},
	}
	for _, c := range cases {
		out, err := runHook(t, c.remote, c.stdin, c.gh, c.code, c.env...)
		if (err != nil) != c.refuse {
			t.Errorf("%s: err = %v, want refuse=%v\n%s", c.name, err, c.refuse, out)
		}
		if c.want != "" && !strings.Contains(out, c.want) {
			t.Errorf("%s: output lacks %q:\n%s", c.name, c.want, out)
		}
	}
}
