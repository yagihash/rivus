package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runMerge runs scripts/merge-pr.sh 7 with a fake `gh`: `pr view` gives a clean PR, `run list` prints runs (exit runsCode),
// and `pr merge` leaves a marker file. It returns the output, the error, and whether the merge was called.
func runMerge(t *testing.T, runs string, runsCode int, env ...string) (string, error, bool) {
	t.Helper()
	bin := t.TempDir()
	for _, tool := range []string{"bash", "python3", "cat", "cut", "touch", "env"} {
		if p, err := exec.LookPath(tool); err == nil {
			os.Symlink(p, filepath.Join(bin, tool))
		}
	}
	marker := filepath.Join(bin, "merged")
	pr := `{"state":"OPEN","isDraft":false,"baseRefName":"main","headRefName":"feat","mergeStateStatus":"CLEAN","body":"","files":[{"path":"a.go"}],"commits":[]}`
	script := `#!/bin/bash
case "$1 $2" in
"repo view") echo o/r;;
"pr view") cat <<'EOF2'
` + pr + `
EOF2
;;
"run list") cat <<'EOF2'
` + runs + `
EOF2
exit ` + string(rune('0'+runsCode)) + `
;;
"pr merge") touch ` + marker + `;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	sh, _ := filepath.Abs("merge-pr.sh")
	cmd := exec.Command("bash", sh, "7")
	cmd.Env = append([]string{"PATH=" + bin}, env...)
	out, err := cmd.CombinedOutput()
	_, statErr := os.Stat(marker)
	return string(out), err, statErr == nil
}

func run(status, conclusion, sha, url string) string {
	return `{"status":"` + status + `","conclusion":"` + conclusion + `","headSha":"` + sha + `","url":"` + url + `"}`
}

func TestMergeGuardMainCI(t *testing.T) {
	ok := run("completed", "success", "aaa", "https://x/ok")
	bad := run("completed", "failure", "bbb", "https://x/bad")
	going := run("in_progress", "", "ccc", "https://x/going")
	cases := []struct {
		name   string
		runs   string
		code   int
		env    []string
		merged bool
		want   []string
	}{
		{"head success", "[" + ok + "," + bad + "]", 0, nil, true, []string{"merging"}},
		{"head failure", "[" + bad + "," + ok + "]", 0, nil, false, []string{"https://x/bad", "bbb"}},
		{"head running, before it failure", "[" + going + "," + bad + "]", 0, nil, false, []string{"https://x/bad"}},
		{"head running, before it success", "[" + going + "," + ok + "]", 0, nil, true, []string{"merging"}},
		{"query fails", "", 1, nil, false, []string{"cannot check"}},
		{"no runs", "[]", 0, nil, false, []string{"cannot check"}},
		{"fixes main", "[" + bad + "]", 0, []string{"RIVUS_MERGE_FIXES_MAIN=1"}, true, []string{"RIVUS_MERGE_FIXES_MAIN=1 で通す", "https://x/bad"}},
		{"fixes main, query fails", "", 1, []string{"RIVUS_MERGE_FIXES_MAIN=1"}, true, []string{"RIVUS_MERGE_FIXES_MAIN=1 で通す"}},
	}
	for _, c := range cases {
		out, err, merged := runMerge(t, c.runs, c.code, c.env...)
		if merged != c.merged || (err == nil) != c.merged {
			t.Errorf("%s: err = %v, merged = %v, want merged=%v\n%s", c.name, err, merged, c.merged, out)
		}
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s: output lacks %q:\n%s", c.name, w, out)
			}
		}
	}
}
