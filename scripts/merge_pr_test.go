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
// The files `gh pr diff --name-only` prints, and the guardrail list the script reads (testGuardList "" = a copy of the real
// .github/guardrail-paths; "-" = no list file; "dir" = a directory where the file should be; else the content). Tests set and restore them.
var (
	testChanged = "a.go"
	// testPrevious are the old paths of the renamed files that the API of the files of a pull request gives (previous_filename), and
	// testAPIFails makes that call fail.
	testPrevious  = ""
	testAPIFails  = false
	testGuardList = ""
)

// lastMergeArgs is what the fake `gh pr merge` of the last runMergePR was given.
var lastMergeArgs string

const okChecks = `[{"name":"build","bucket":"pass"},{"name":"ci-ok","bucket":"pass"},{"name":"main-ci","bucket":"pass"}]`

func apiFails() string {
	if testAPIFails {
		return "1"
	}
	return "0"
}

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
"pr diff") cat <<'EOF3'
` + testChanged + `
EOF3
;;
"api repos/o/r/pulls/7/files") if [ ` + apiFails() + ` = 1 ]; then exit 1; fi
cat <<'EOF4'
` + testPrevious + `
EOF4
;;
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
"pr merge") echo "$@" > ` + marker + `;;
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
	root := t.TempDir()
	sdir := filepath.Join(root, "scripts")
	os.MkdirAll(sdir, 0o755)
	os.MkdirAll(filepath.Join(root, ".github"), 0o755)
	list := filepath.Join(root, ".github", "guardrail-paths")
	switch testGuardList {
	case "":
		real, err := os.ReadFile("../.github/guardrail-paths")
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(list, real, 0o644)
	case "-":
	case "dir":
		os.MkdirAll(list, 0o755)
	default:
		os.WriteFile(list, []byte(testGuardList), 0o644)
	}
	sh := filepath.Join(sdir, "merge-pr.sh")
	os.WriteFile(sh, src, 0o755)
	os.WriteFile(filepath.Join(sdir, "zoo-loosened-check.sh"), []byte("#!/bin/bash\necho zoo check says no\nexit "+string(rune('0'+zooCode))+"\n"), 0o755)
	cmd := exec.Command("bash", sh, "7")
	cmd.Env = append([]string{"PATH=" + bin}, env...)
	out, err := cmd.CombinedOutput()
	args, statErr := os.ReadFile(marker)
	lastMergeArgs = strings.TrimSpace(string(args))
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
		{"pass and skipping", `[{"name":"a","bucket":"pass"},{"name":"b","bucket":"skipping"},{"name":"ci-ok","bucket":"pass"},{"name":"main-ci","bucket":"pass"}]`, true, "merging"},
		{"one pending", `[{"name":"a","bucket":"pending"},{"name":"ci-ok","bucket":"pass"}]`, false, "a (pending)"},
		{"one fail", `[{"name":"a","bucket":"fail"},{"name":"ci-ok","bucket":"pass"}]`, false, "a (fail)"},
		{"one cancel", `[{"name":"a","bucket":"cancel"},{"name":"ci-ok","bucket":"pass"}]`, false, "a (cancel)"},
		{"ci-ok pending", `[{"name":"a","bucket":"pass"},{"name":"ci-ok","bucket":"pending"}]`, false, "ci-ok (pending)"},
		{"no ci-ok", `[{"name":"a","bucket":"pass"}]`, false, "no passing ci-ok"},
		{"ci-ok skipping", `[{"name":"ci-ok","bucket":"skipping"}]`, false, "no passing ci-ok"},
		{"no checks", `[]`, false, "no passing ci-ok"},
		{"unreadable", ``, false, "cannot read the checks"},
		// main-ci is required by name: the check run that main-ci.yml writes on the head of the PR (G1). Absent, it is not "all passed": the workflow may not have started yet.
		{"main-ci pass", `[{"name":"ci-ok","bucket":"pass"},{"name":"main-ci","bucket":"pass"}]`, true, "merging"},
		{"main-ci fail", `[{"name":"ci-ok","bucket":"pass"},{"name":"main-ci","bucket":"fail"}]`, false, "main-ci (fail)"},
		{"main-ci pending", `[{"name":"ci-ok","bucket":"pass"},{"name":"main-ci","bucket":"pending"}]`, false, "main-ci (pending)"},
		{"main-ci missing", `[{"name":"a","bucket":"pass"},{"name":"ci-ok","bucket":"pass"}]`, false, "no passing main-ci"},
		{"main-ci skipping", `[{"name":"ci-ok","bucket":"pass"},{"name":"main-ci","bucket":"skipping"}]`, false, "no passing main-ci"},
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

const refused = "この PR はガードレールの部品を変えるので、ユーザーのマージ"

func withGuard(t *testing.T, changed, list string) {
	t.Helper()
	oc, ol := testChanged, testGuardList
	testChanged, testGuardList = changed, list
	t.Cleanup(func() { testChanged, testGuardList = oc, ol })
}

var okRuns = "[" + run("completed", "success", "aaa", "https://x/ok") + "]"

// Each part the user keeps has a pattern in the list, and a PR that changes a file of that part is refused (one part at a time).
func TestMergeGuardrailPaths(t *testing.T) {
	parts := []struct{ pattern, file string }{
		{".github/workflows/", ".github/workflows/ci.yml"},
		{".githooks/", ".githooks/pre-push"},
		{".claude/hooks/", ".claude/hooks/x.sh"},
		{".claude/settings*", ".claude/settings.json"},
		{".claude/settings*", ".claude/settings.local.json"},
		{".claude/prompts/", ".claude/prompts/p.md"},
		{".regomark/", ".regomark/policy/wording.rego"},
		{".github/ghmint/", ".github/ghmint/a.rego"},
		{".github/rulesets/", ".github/rulesets/main.json"},
		{"scripts/merge-pr.sh", "scripts/merge-pr.sh"},
		{"scripts/zoo-loosened-check.sh", "scripts/zoo-loosened-check.sh"},
		{"scripts/main-ci*", "scripts/main-ci.sh"},
		{".github/guardrail-paths", ".github/guardrail-paths"},
	}
	real, err := os.ReadFile("../.github/guardrail-paths")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range parts {
		if !strings.Contains("\n"+string(real), "\n"+p.pattern+"\n") {
			t.Errorf("the list lacks %q", p.pattern)
		}
		withGuard(t, "a.go\n"+p.file+"\n", "")
		out, err, merged := runMerge(t, okRuns, 0)
		if err == nil || merged || !strings.Contains(out, refused) || !strings.Contains(out, p.file) {
			t.Errorf("%s: err = %v, merged = %v\n%s", p.file, err, merged, out)
		}
	}
}

func TestMergeGuardrailPathsOutside(t *testing.T) {
	for _, f := range []string{"a.go", "docs/WORKFLOW.md", "scripts/shard.sh", ".github/release.yml", ".claude/work/QUEUE.md", "github/workflows/x.yml", "x/.githooks/y"} {
		withGuard(t, f, "")
		out, err, merged := runMerge(t, okRuns, 0)
		if err != nil || !merged {
			t.Errorf("%s: err = %v, merged = %v\n%s", f, err, merged, out)
		}
	}
}

// A missing, unreadable, empty or comment-only list refuses every PR; so does a changed-file list that cannot be read.
func TestMergeGuardrailListFailsSafe(t *testing.T) {
	for name, list := range map[string]string{"missing": "-", "unreadable": "dir", "empty": "\n", "comments only": "# x\n"} {
		withGuard(t, "a.go", list)
		out, err, merged := runMerge(t, okRuns, 0)
		if err == nil || merged || !strings.Contains(out, "guardrail list") {
			t.Errorf("%s: err = %v, merged = %v\n%s", name, err, merged, out)
		}
	}
}

// The merge is for the commit that was looked at (headRefOid of the PR JSON): a push after the checks is refused by GitHub.
func TestMergeMatchesHeadCommit(t *testing.T) {
	ok := run("completed", "success", "aaa", "https://x/ok")
	if out, err, merged := runMerge(t, "["+ok+"]", 0); err != nil || !merged {
		t.Fatalf("not merged: %v\n%s", err, out)
	}
	if !strings.Contains(lastMergeArgs, "--match-head-commit abc") {
		t.Errorf("gh pr merge was given %q: --match-head-commit with the head commit of the PR is missing", lastMergeArgs)
	}
}

// A file of the guardrail that is renamed is not in `gh pr diff --name-only` by its old path (only the new one is): the old path is read
// from previous_filename, and when that cannot be read nothing is merged.
func TestMergeGuardrailRenamedAway(t *testing.T) {
	oc, op, of := testChanged, testPrevious, testAPIFails
	t.Cleanup(func() { testChanged, testPrevious, testAPIFails = oc, op, of })
	testChanged, testPrevious = "scripts/x.sh", "scripts/merge-pr.sh"
	out, err, merged := runMerge(t, okRuns, 0)
	if err == nil || merged || !strings.Contains(out, refused) || !strings.Contains(out, "scripts/merge-pr.sh") {
		t.Errorf("a guardrail file moved away: err = %v, merged = %v\n%s", err, merged, out)
	}
	testChanged, testPrevious = "a.go", ""
	if out, err, merged := runMerge(t, okRuns, 0); err != nil || !merged {
		t.Errorf("no rename: err = %v, merged = %v\n%s", err, merged, out)
	}
	testAPIFails = true
	out, err, merged = runMerge(t, okRuns, 0)
	if err == nil || merged || !strings.Contains(out, "renamed") {
		t.Errorf("the list of renames cannot be read: err = %v, merged = %v\n%s", err, merged, out)
	}
}
