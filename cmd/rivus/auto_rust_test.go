package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/report"
)

// autoProject is a Go module with a Rust project beside it; its build.rs makes a file that says it ran.
func autoProject(t *testing.T, withGo bool) (dir, ran string) {
	t.Helper()
	dir = t.TempDir()
	ran = filepath.Join(t.TempDir(), "BUILD-RS-RAN")
	if withGo {
		write(t, filepath.Join(dir, "go.mod"), "module example.com/mix\n\ngo 1.21\n")
		write(t, filepath.Join(dir, "main.go"), "package main\n\nfunc a() {}\n\nfunc b() {}\n\nfunc main() { a() }\n")
	}
	write(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"r\"\nversion = \"0.1.0\"\n")
	write(t, filepath.Join(dir, "build.rs"), "fn main() { std::fs::write(\""+ran+"\", \"x\").unwrap(); }\n")
	write(t, filepath.Join(dir, "src", "lib.rs"), "")
	return dir, ran
}

func autoOpts(dir string, terminal bool, in string, yes bool) (*analyzeOpts, *int) {
	o := &analyzeOpts{}
	o.dir, o.lang, o.yes = dir, "auto", yes
	asks := 0
	o.isTerminal = func() bool { asks++; return terminal }
	o.confirmIn = strings.NewReader(in)
	o.sandboxCheck, o.installCheck = func() error { return nil }, func() error { return nil }
	return o, &asks
}

func TestAutoRustAnswers(t *testing.T) {
	dir, ran := autoProject(t, true)
	for _, c := range []struct {
		name      string
		terminal  bool
		in        string
		yes       bool
		wantRust  bool
		wantSkips []report.SkippedLanguage
	}{
		{"yes", true, "y\n", false, true, nil},
		{"enter is yes", true, "\n", false, true, nil},
		{"no", true, "n\n", false, false, []report.SkippedLanguage{{Lang: "rust", Why: "declined"}}},
		{"end of input is no", true, "", false, false, []report.SkippedLanguage{{Lang: "rust", Why: "declined"}}},
		{"no terminal, no -yes", false, "y\n", false, false, []report.SkippedLanguage{{Lang: "rust", Why: "no-terminal"}}},
		{"no terminal, -yes", false, "", true, true, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			o, _ := autoOpts(dir, c.terminal, c.in, c.yes)
			got, err := o.languages()
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"go"}
			if c.wantRust {
				want = []string{"go", "rust"}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("languages = %v, want %v", got, want)
			}
			if !reflect.DeepEqual(o.skipped, c.wantSkips) {
				t.Errorf("skipped = %v, want %v", o.skipped, c.wantSkips)
			}
		})
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("the build script ran: nothing here reads the project")
	}
}

// The answer is decided once: the commands call languages() more than once.
func TestAutoRustIsAskedOnce(t *testing.T) {
	dir, _ := autoProject(t, true)
	o, asks := autoOpts(dir, true, "n\n", false)
	first, _ := o.languages()
	second, _ := o.languages()
	if *asks != 1 || !reflect.DeepEqual(first, second) || len(o.skipped) != 1 {
		t.Errorf("asked %d times, %v then %v, skipped %v", *asks, first, second, o.skipped)
	}
}

func TestAutoRustAlone(t *testing.T) {
	dir, _ := autoProject(t, false)
	// no: nothing is read, and the run ends with its own code
	o, _ := autoOpts(dir, true, "n\n", false)
	_, err := o.languages()
	var d *declinedError
	if !errors.As(err, &d) {
		t.Fatalf("declined alone: %v", err)
	}
	if exitCode(err) != exitDeclined || exitCode(errors.New("x")) != 1 {
		t.Errorf("exit codes: %d, %d", exitCode(err), exitCode(errors.New("x")))
	}
	if exitDeclined == 0 || exitDeclined == 1 || exitDeclined == 2 || exitDeclined == 130 {
		t.Errorf("exit code %d is one that is in use", exitDeclined)
	}
	// no terminal and no -yes is not "no": it is the error, and it says what to give
	o, _ = autoOpts(dir, false, "", false)
	_, err = o.languages()
	if err == nil || errors.As(err, &d) || !strings.Contains(err.Error(), "-lang rust or -yes") {
		t.Errorf("no terminal alone: %v", err)
	}
	// yes
	o, _ = autoOpts(dir, false, "", true)
	if got, err := o.languages(); err != nil || !reflect.DeepEqual(got, []string{"rust"}) {
		t.Errorf("-yes alone: %v, %v", got, err)
	}
	// -lang rust asks nothing
	o, asks := autoOpts(dir, true, "n\n", false)
	o.lang = "rust"
	if got, err := o.languages(); err != nil || !reflect.DeepEqual(got, []string{"rust"}) || *asks != 0 {
		t.Errorf("-lang rust: %v, %v, asked %d", got, err, *asks)
	}
}

func TestAutoRustNotAskedWhenItCannotBeRead(t *testing.T) {
	dir, _ := autoProject(t, true)
	// no sandbox: not asked, not read, even with -yes and -unsafe-host-exec
	o, asks := autoOpts(dir, true, "y\n", true)
	o.unsafeHostExec = true
	o.sandboxCheck = func() error { return errors.New("none") }
	got, err := o.languages()
	if err != nil || !reflect.DeepEqual(got, []string{"go"}) || *asks != 0 || !reflect.DeepEqual(o.skipped, []report.SkippedLanguage{{Lang: "rust", Why: "no-sandbox"}}) {
		t.Errorf("no sandbox: %v, %v, asked %d, %v", got, err, *asks, o.skipped)
	}
	// not installed: said before it is asked
	o, asks = autoOpts(dir, true, "y\n", false)
	o.installCheck = func() error { return errors.New("no rivus-rust") }
	got, err = o.languages()
	if err != nil || !reflect.DeepEqual(got, []string{"go"}) || *asks != 0 || !reflect.DeepEqual(o.skipped, []report.SkippedLanguage{{Lang: "rust", Why: "not-installed"}}) {
		t.Errorf("not installed: %v, %v, asked %d, %v", got, err, *asks, o.skipped)
	}
	// alone, both are errors (not "declined")
	alone, _ := autoProject(t, false)
	for _, mk := range []func(o *analyzeOpts){
		func(o *analyzeOpts) { o.sandboxCheck = func() error { return errors.New("none") } },
		func(o *analyzeOpts) { o.installCheck = func() error { return errors.New("no rivus-rust") } },
	} {
		o, _ := autoOpts(alone, true, "y\n", true)
		mk(o)
		var d *declinedError
		if _, err := o.languages(); err == nil || errors.As(err, &d) {
			t.Errorf("alone: %v", err)
		}
	}
}

// The Rust project that was not read is in the result and in the index, and not among the packages that failed;
// no Rust code ran.
func TestSkippedLanguagesAreInTheResult(t *testing.T) {
	dir, ran := autoProject(t, true)
	o, _ := autoOpts(dir, true, "n\n", false)
	o.noBuiltin, o.noProject = true, true
	rep, err := o.analyze([]string{"./..."})
	if err != nil {
		t.Fatal(err)
	}
	u := rep.Summary.Unread
	if u == nil || !reflect.DeepEqual(u.SkippedLanguages, []report.SkippedLanguage{{Lang: "rust", Why: "declined"}}) {
		t.Fatalf("summary.unread = %+v", u)
	}
	if u.Packages != 0 {
		t.Errorf("a language that was not read counted as a package: %+v", u)
	}
	for _, e := range rep.LoadErrors {
		if e.Package == "rust" {
			t.Errorf("rust is among the load errors: %+v", e)
		}
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("the build script ran")
	}
	raw, _ := json.Marshal(rep.Summary)
	if !strings.Contains(string(raw), `"skipped_languages":[{"lang":"rust","why":"declined"}]`) {
		t.Errorf("%s", raw)
	}
}

// Through the binary (the input is not a terminal): the index says it, "no path" is not exhaustive, and the build script did not run.
func TestIndexWithoutRustIsNotExhaustive(t *testing.T) {
	dir, ran := autoProject(t, true)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, dir, nil, "index", "-graph", db, "./...")
	if _, err := os.Stat(ran); err == nil {
		t.Error("the build script ran")
	}
	d, err := graphdb.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	var u graphdb.Unread
	if err := json.Unmarshal([]byte(d.Meta("unread")), &u); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if len(u.SkippedLanguages) != 1 || u.SkippedLanguages[0].Lang != "rust" {
		t.Fatalf("meta.unread = %+v", u)
	}
	if w := u.SkippedLanguages[0].Why; w != "no-terminal" && !(w == "no-sandbox" && runtime.GOOS != "darwin") && w != "not-installed" {
		t.Errorf("why = %q", w)
	}
	if u.FailedPackages != 0 {
		t.Errorf("failed packages: %+v", u)
	}
	out := runIn(t, binary, dir, nil, "graph", "path", "-graph", db, "--from", "example.com/mix.b", "--to", "example.com/mix.a")
	var p struct {
		Reachable  bool `json:"reachable"`
		Exhaustive bool `json:"exhaustive"`
		Because    *struct {
			Skipped struct {
				Total int    `json:"total"`
				Why   string `json:"why"`
			} `json:"skipped_languages"`
		} `json:"not_exhaustive_because"`
	}
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if p.Reachable || p.Exhaustive || p.Because == nil || p.Because.Skipped.Total != 1 || !strings.Contains(p.Because.Skipped.Why, "rust was not read") {
		t.Errorf("%s", out)
	}
}

// With -yes the index holds what the answer was; an index made with Rust is not the index made without it, and the key says so.
func TestIndexWithAndWithoutRustDiffer(t *testing.T) {
	dir, _ := autoProject(t, true)
	stream := filepath.Join(t.TempDir(), "empty.jsonl")
	script := filepath.Join(t.TempDir(), "extractor.sh")
	write(t, script, "#!/bin/sh\ncat "+stream+"\n")
	os.Chmod(script, 0o755)
	write(t, stream, emptyIR(t))
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, dir, []string{"RIVUS_UNSAFE_HOST_EXEC=1"}, "index", "-graph", db, "-lang", "go,rust", "-rust-extractor", script, "./...")
	info := func(args ...string) string {
		return string(runIn(t, binary, dir, nil, append([]string{"graph", "info", "-graph", db, "-C", dir, "-rust-extractor", script}, args...)...))
	}
	if out := info("-lang", "go"); !strings.Contains(out, `"fresh": false`) || !strings.Contains(out, "lang") {
		t.Errorf("an index made with Rust, held against the code read without it: %s", out)
	}
	if out := info("-lang", "go,rust"); !strings.Contains(out, `"fresh": true`) {
		t.Errorf("the same languages: %s", out)
	}
}

func emptyIR(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := ir.Write(&buf, &ir.Program{}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
