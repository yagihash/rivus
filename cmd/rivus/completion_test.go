package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// zshScript is the script that the binary writes.
func zshScript(t *testing.T) string {
	t.Helper()
	out, err := exec.Command(binary, "completion", "zsh").Output()
	if err != nil {
		t.Fatalf("rivus completion zsh: %v", err)
	}
	return string(out)
}

// zshBody is the text of the function fn in the script.
func zshBody(t *testing.T, script, fn string) string {
	t.Helper()
	_, rest, ok := strings.Cut(script, "\n"+fn+"() {\n")
	if !ok {
		t.Fatalf("the script has no function %s", fn)
	}
	body, _, _ := strings.Cut(rest, "\n}\n")
	return body
}

// The script has every command and subcommand of the table, and every flag of every FlagSet (listed on an empty flag set, as the generator lists them).
func TestZshCompletionHasTheTable(t *testing.T) {
	t.Parallel()
	script := zshScript(t)
	if !strings.HasPrefix(script, "#compdef rivus\n") || !strings.Contains(script, "\ncompdef _rivus rivus\n") {
		t.Errorf("the script does not start with #compdef rivus and end with compdef _rivus rivus")
	}
	var walk func(cs []*command, path []string)
	walk = func(cs []*command, path []string) {
		parent := zshBody(t, script, zshFunc(path))
		for _, c := range cs {
			p := append(append([]string{}, path...), c.name)
			if !strings.Contains(parent, "'"+c.name+":") {
				t.Errorf("%s does not list %q", zshFunc(path), c.name)
			}
			if len(c.sub) > 0 {
				walk(c.sub, p)
				continue
			}
			body := zshBody(t, script, zshFunc(p))
			if c.flags == nil {
				continue
			}
			fs := flag.NewFlagSet(c.name, flag.ContinueOnError)
			c.flags(fs)
			fs.VisitAll(func(f *flag.Flag) {
				if !strings.Contains(body, "'-"+f.Name+"[") && !strings.Contains(body, "'-"+f.Name+"=[") {
					t.Errorf("%s does not complete -%s", zshFunc(p), f.Name)
				}
			})
			for _, f := range []string{helpAllFlag, "h"} {
				if !strings.Contains(body, "'-"+f+"[") {
					t.Errorf("%s does not complete -%s", zshFunc(p), f)
				}
			}
		}
	}
	walk(commands, nil)
}

// The values of a flag of fixed values are the ones of the list that the check reads; the flags for diagnosis say so; a directory and a file are
// completed as such.
func TestZshCompletionValues(t *testing.T) {
	t.Parallel()
	script := zshScript(t)
	analyze := zshBody(t, script, "_rivus_analyze")
	for _, want := range []string{
		"'-lang=[", "(go dart rust auto)'", "(summary snippet)'", "(reanalyses time functions elems)'", "'-C=[directory to analyse from (default \".\")]:dir:_files -/'",
		"'-rules=[additional rule file (repeatable)]:file:_files'", "[(diagnostic) ",
	} {
		if !strings.Contains(analyze, want) {
			t.Errorf("_rivus_analyze lacks %q", want)
		}
	}
	if !strings.Contains(zshBody(t, script, "_rivus_sites"), "(source sink)'") {
		t.Errorf("_rivus_sites does not complete the values of -role")
	}
	// the usual flags come before the ones for diagnosis
	if i, j := strings.Index(analyze, "'-sites["), strings.Index(analyze, "(diagnostic)"); i < 0 || j < 0 || i > j {
		t.Errorf("the flags for diagnosis are not after the usual flags")
	}
	if !strings.Contains(zshBody(t, script, "_rivus_completion"), "zsh") {
		t.Errorf("completion does not list zsh")
	}
}

func TestZshDescriptionEscapes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want string }{
		{"a: b [c] d", `a\: b \[c\] d`},
		{`it's`, `it's`},
	} {
		if got := zshSpecText(c.in); got != c.want {
			t.Errorf("zshSpecText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := zshQuote("it's"); got != `'it'\''s'` {
		t.Errorf("zshQuote = %s", got)
	}
	if got := firstSentence("one two. three four"); got != "one two" {
		t.Errorf("firstSentence = %q", got)
	}
	if got := firstSentence(strings.Repeat("word ", 40)); len(got) > maxDescription+3 || !strings.HasSuffix(got, "...") {
		t.Errorf("a long usage is cut: %q", got)
	}
}

// The script is valid zsh, and compinit can load it (the functions are defined).
func TestZshCompletionSyntax(t *testing.T) {
	t.Parallel()
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		skipOrFail(t, "zsh")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "_rivus")
	if err := os.WriteFile(file, []byte(zshScript(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(zsh, "-n", file).CombinedOutput(); err != nil {
		t.Fatalf("zsh -n: %v\n%s", err, out)
	}
	load := "autoload -U compinit && compinit -u -d " + filepath.Join(dir, "zcompdump") + " && source " + file +
		" && whence -w _rivus_analyze _rivus_graph_callers _rivus_debug_extract >/dev/null && print ${_comps[rivus]}"
	out, err := exec.Command(zsh, "-f", "-c", load).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "_rivus" {
		t.Fatalf("source: %v, %q", err, out)
	}
}

// What the script holds comes from the build, not from where it was built or where it runs (the script is made while a formula builds rivus): the
// directory, the home directory and the path of the program are not in it.
func TestZshCompletionHasNoLocalPaths(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	home := filepath.Join(t.TempDir(), "a-home-directory")
	cmd := exec.Command(binary, "completion", "zsh")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CACHE_HOME="+filepath.Join(home, ".cache"))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	script := stdout.String()
	wd, _ := os.Getwd()
	exe, _ := os.Executable()
	for name, v := range map[string]string{"the directory it ran in": cwd, "$HOME": home, "the path of the program": binary, "the directory of the tests": wd, "the test binary": exe, "the temporary directory": os.TempDir()} {
		if v != "" && v != "/" && strings.Contains(script, v) {
			t.Errorf("the script holds %s (%s)", name, v)
		}
	}
	if u, err := os.UserHomeDir(); err == nil && len(u) > 1 && strings.Contains(script, u) {
		t.Errorf("the script holds the home directory of the user (%s)", u)
	}
}

// It is quick and needs nothing: no extractor, no network, no directory to read (a PATH with nothing in it).
func TestZshCompletionIsQuick(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "completion", "zsh")
	cmd.Dir = t.TempDir()
	cmd.Env = []string{"PATH=" + t.TempDir()}
	start := time.Now()
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		t.Fatalf("rivus completion zsh: %v after %v", err, time.Since(start))
	}
}

// "rivus completion" says which shells it has and ends with 2; -h ends with 0; a shell that is not there, or an argument, is an error.
func TestCompletionCommand(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cmd := exec.Command(binary, "completion")
	cmd.Stderr = &stderr
	if ee, ok := cmd.Run().(*exec.ExitError); !ok || ee.ExitCode() != 2 || !strings.Contains(stderr.String(), "zsh") {
		t.Errorf("rivus completion: %v, %q", ee, stderr.String())
	}
	for _, args := range [][]string{{"completion", "-h"}, {"completion", "zsh", "-h"}} {
		if out, err := exec.Command(binary, args...).CombinedOutput(); err != nil {
			t.Errorf("rivus %v: %v, %s", args, err, out)
		}
	}
	for _, args := range [][]string{{"completion", "bash"}, {"completion", "zsh", "extra"}} {
		if out, err := exec.Command(binary, args...).CombinedOutput(); err == nil {
			t.Errorf("rivus %v ended with 0: %s", args, out)
		}
	}
	out, _ := exec.Command(binary, "help").Output()
	if !strings.Contains(string(out), "\n  completion ") {
		t.Errorf("rivus help does not list completion:\n%s", out)
	}
}
