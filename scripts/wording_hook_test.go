package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runWordingHook runs .claude/hooks/check-wording.sh for a write of file, with a fake `regomark` first on PATH: it
// prints "doc.md:3: [WORD001] x" and exits 1 when the copy it is given (doc.md) holds the word BAD, and exits 0
// otherwise; with noRegomark there is none on PATH. project is CLAUDE_PROJECT_DIR.
func runWordingHook(t *testing.T, file, project string, noRegomark bool) (string, error) {
	t.Helper()
	bin := t.TempDir()
	for _, tool := range []string{"bash", "git", "jq", "sed", "head", "cp", "mkdir", "mktemp", "dirname", "grep", "cat"} {
		p, err := exec.LookPath(tool)
		if err != nil {
			t.Fatalf("the hook needs %s", tool)
		}
		os.Symlink(p, filepath.Join(bin, tool))
	}
	if !noRegomark {
		script := "#!/bin/bash\n[ -f .regomark.yaml ] && [ -d policy ] || { echo 'no config'; exit 2; }\n" +
			"if grep -q BAD doc.md; then echo 'doc.md:3: [WORD001] x'; exit 1; fi\nexit 0\n"
		if err := os.WriteFile(filepath.Join(bin, "regomark"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	hook, _ := filepath.Abs("../.claude/hooks/check-wording.sh")
	cmd := exec.Command("bash", hook)
	cmd.Env = []string{"PATH=" + bin, "CLAUDE_PROJECT_DIR=" + project, "TMPDIR=" + t.TempDir(), "HOME=" + t.TempDir()}
	cmd.Stdin = strings.NewReader(`{"tool_input":{"file_path":"` + file + `"}}`)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// wordingProject makes a directory that has the configuration and a policy, as the project has; with repo it is a
// git repository.
func wordingProject(t *testing.T, repo bool) string {
	t.Helper()
	dir := t.TempDir()
	// macOS: the temporary directory is behind a link, and git gives the path without it
	if p, err := filepath.EvalSymlinks(dir); err == nil {
		dir = p
	}
	if repo {
		if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
	}
	os.MkdirAll(filepath.Join(dir, ".regomark", "policy"), 0o755)
	os.WriteFile(filepath.Join(dir, ".regomark", "policy", "wording.rego"), []byte("package wording\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".regomark.yaml"), []byte("documents:\n  - \"docs/**/*.md\"\npolicy: .regomark/policy\nentrypoint: wording\n"), 0o644)
	return dir
}

func writeDoc(t *testing.T, path, text string) string {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Every .md that is written is checked: one in the documents of the configuration, one in the repository that the
// configuration does not list, and one outside any repository (the notes between sessions), with the project's policy.
func TestWordingHookChecksEveryMarkdown(t *testing.T) {
	project := wordingProject(t, true)
	outside := t.TempDir()
	if p, err := filepath.EvalSymlinks(outside); err == nil {
		outside = p
	}
	otherRepo := wordingProject(t, true) // a repository with its own configuration: its policy is the one used
	cases := []struct {
		name string
		file string
		bad  bool
	}{
		{"listed document", filepath.Join(project, "docs", "a.md"), true},
		{"listed document, clean", filepath.Join(project, "docs", "b.md"), false},
		{"in the repository, not listed", filepath.Join(project, ".claude", "reviews", "r.md"), true},
		{"outside any repository", filepath.Join(outside, "inbox", "c.md"), true},
		{"outside any repository, clean", filepath.Join(outside, "inbox", "d.md"), false},
		{"another repository with a configuration", filepath.Join(otherRepo, "notes.md"), true},
	}
	for _, c := range cases {
		text := "# t\n\nfine\n"
		if c.bad {
			text = "# t\n\nBAD\n"
		}
		writeDoc(t, c.file, text)
		out, err := runWordingHook(t, c.file, project, false)
		if c.bad {
			ee, ok := err.(*exec.ExitError)
			if !ok || ee.ExitCode() != 2 {
				t.Errorf("%s: err = %v, want exit 2\n%s", c.name, err, out)
				continue
			}
			// the line names the file that was written, not the copy
			if !strings.Contains(out, c.file+":3: [WORD001]") || strings.Contains(out, "doc.md:") {
				t.Errorf("%s: the output does not name the file:\n%s", c.name, out)
			}
		} else if err != nil {
			t.Errorf("%s: err = %v, want none\n%s", c.name, err, out)
		}
	}
}

// What is not checked, and says nothing: a file that is not .md, a .md that is gone, and a .md when neither its
// repository nor the project has a configuration.
func TestWordingHookLeavesTheRestAlone(t *testing.T) {
	project := wordingProject(t, true)
	bare := t.TempDir() // no configuration
	for _, c := range []struct{ name, file, project string }{
		{"not markdown", writeDoc(t, filepath.Join(project, "a.go"), "BAD\n"), project},
		{"the file is gone", filepath.Join(project, "docs", "gone.md"), project},
		{"no configuration anywhere", writeDoc(t, filepath.Join(bare, "x.md"), "BAD\n"), bare},
	} {
		out, err := runWordingHook(t, c.file, c.project, false)
		if err != nil || strings.TrimSpace(out) != "" {
			t.Errorf("%s: err = %v, output %q, want nothing", c.name, err, out)
		}
	}
}

// Without regomark the hook does not fail the write: it says that the check was skipped, and how to install it.
func TestWordingHookWithoutRegomark(t *testing.T) {
	project := wordingProject(t, true)
	file := writeDoc(t, filepath.Join(project, "docs", "a.md"), "BAD\n")
	out, err := runWordingHook(t, file, project, true)
	if err != nil {
		t.Fatalf("err = %v, want none\n%s", err, out)
	}
	if !strings.Contains(out, "regomark") || !strings.Contains(out, "systemMessage") {
		t.Errorf("the output does not say that the check was skipped:\n%s", out)
	}
}
