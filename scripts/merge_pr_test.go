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
const okChecks = `[{"name":"build","bucket":"pass"},{"name":"ci-ok","bucket":"pass"}]`

func runMerge(t *testing.T, runs string, runsCode int, env ...string) (string, error, bool) {
	return runMergeChecks(t, okChecks, runs, runsCode, env...)
}

// runMergeChecks is runMerge with the JSON that `gh pr checks` prints (it exits 8 or 1 when something is pending or failed).
func runMergeChecks(t *testing.T, checks, runs string, runsCode int, env ...string) (string, error, bool) {
	return runMergePR(t, `"commits":[]`, 0, checks, runs, runsCode, env...)
}

// runMergePR is runMergeChecks with the "commits" member of the PR JSON and the exit code of a fake zoo-loosened-check.sh
// (the script is copied to a temporary folder together with it; `git` is a fake that succeeds).
func runMergePR(t *testing.T, commits string, zooCode int, checks, runs string, runsCode int, env ...string) (string, error, bool) {
	t.Helper()
	bin := t.TempDir()
	for _, tool := range []string{"bash", "python3", "cat", "cut", "touch", "env", "sed"} {
		if p, err := exec.LookPath(tool); err == nil {
			os.Symlink(p, filepath.Join(bin, tool))
		}
	}
	marker := filepath.Join(bin, "merged")
	pr := `{"state":"OPEN","isDraft":false,"baseRefName":"main","headRefName":"feat","mergeStateStatus":"CLEAN","body":"","headRefOid":"abc","files":[{"path":"a.go"}],` + commits + `}`
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
"pr checks") cat <<'EOF2'
` + checks + `
EOF2
exit 8
;;
"pr merge") touch ` + marker + `;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("merge-pr.sh")
	if err != nil {
		t.Fatal(err)
	}
	sdir := t.TempDir()
	sh := filepath.Join(sdir, "merge-pr.sh")
	os.WriteFile(sh, src, 0o755)
	os.WriteFile(filepath.Join(sdir, "zoo-loosened-check.sh"), []byte("#!/bin/bash\necho zoo check says no\nexit "+string(rune('0'+zooCode))+"\n"), 0o755)
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

func TestMergeGuardChecks(t *testing.T) {
	ok := "[" + run("completed", "success", "aaa", "https://x/ok") + "]"
	cases := []struct {
		name   string
		checks string
		merged bool
		want   string
	}{
		{"pass and skipping", `[{"name":"a","bucket":"pass"},{"name":"b","bucket":"skipping"},{"name":"ci-ok","bucket":"pass"}]`, true, "merging"},
		{"one pending", `[{"name":"a","bucket":"pending"},{"name":"ci-ok","bucket":"pass"}]`, false, "a (pending)"},
		{"one fail", `[{"name":"a","bucket":"fail"},{"name":"ci-ok","bucket":"pass"}]`, false, "a (fail)"},
		{"one cancel", `[{"name":"a","bucket":"cancel"},{"name":"ci-ok","bucket":"pass"}]`, false, "a (cancel)"},
		{"ci-ok pending", `[{"name":"a","bucket":"pass"},{"name":"ci-ok","bucket":"pending"}]`, false, "ci-ok (pending)"},
		{"no ci-ok", `[{"name":"a","bucket":"pass"}]`, false, "no passing ci-ok"},
		{"ci-ok skipping", `[{"name":"ci-ok","bucket":"skipping"}]`, false, "no passing ci-ok"},
		{"no checks", `[]`, false, "no passing ci-ok"},
		{"unreadable", ``, false, "cannot read the checks"},
	}
	for _, c := range cases {
		out, err, merged := runMergeChecks(t, c.checks, ok, 0)
		if merged != c.merged || (err == nil) != c.merged {
			t.Errorf("%s: err = %v, merged = %v, want merged=%v\n%s", c.name, err, merged, c.merged, out)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: output lacks %q:\n%s", c.name, c.want, out)
		}
	}
}

func TestMergeGuardZooLoosened(t *testing.T) {
	ok := "[" + run("completed", "success", "aaa", "https://x/ok") + "]"
	good := "Zoo-Loosened: 理由。ユーザーの承認: 2026-11-24"
	commit := func(body string) string { return `{"messageHeadline":"h","messageBody":"` + body + `"}` }
	cases := []struct {
		name    string
		commits []string
		zooCode int
		merged  bool
		want    string
	}{
		{"one approved line", []string{commit("x\\n" + good)}, 0, true, "merging"},
		{"no line", []string{commit("x")}, 1, true, "merging"},
		{"mada", []string{commit("Zoo-Loosened: 理由。ユーザーの承認: まだ")}, 0, false, "valid approval date"},
		{"no date", []string{commit("Zoo-Loosened: 理由")}, 0, false, "valid approval date"},
		{"bad date", []string{commit("Zoo-Loosened: 理由。ユーザーの承認: 2026-13-45")}, 0, false, "valid approval date"},
		{"future date", []string{commit("Zoo-Loosened: 理由。ユーザーの承認: 2999-01-01")}, 0, false, "valid approval date"},
		{"one of two", []string{commit(good), commit("Zoo-Loosened: 理由。ユーザーの承認: まだ")}, 0, false, "valid approval date"},
		{"two approved", []string{commit(good), commit(good)}, 0, true, "merging"},
		{"check fails", []string{commit(good)}, 1, false, "zoo-loosened-check.sh does not pass"},
	}
	for _, c := range cases {
		commits := `"commits":[` + strings.Join(c.commits, ",") + `]`
		out, err, merged := runMergePR(t, commits, c.zooCode, okChecks, ok, 0)
		if merged != c.merged || (err == nil) != c.merged {
			t.Errorf("%s: err = %v, merged = %v, want merged=%v\n%s", c.name, err, merged, c.merged, out)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: output lacks %q:\n%s", c.name, c.want, out)
		}
	}
}
