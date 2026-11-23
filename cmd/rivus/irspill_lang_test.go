package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
)

// The IR of Dart and of Rust is read from the extractor a line at a time into the file (ADR-123), and the result is
// the same as with the IR in memory: the findings, the uncertainties, the summary, the stats and the edges.

// irspillLangRun analyses the project in dir with the extractor of the language, with the IR in memory or in the file.
func irspillLangRun(t *testing.T, spill bool, dir string, args ...string) acceptResult {
	t.Helper()
	fs := flag.NewFlagSet("irspill-lang", flag.ContinueOnError)
	var o analyzeOpts
	o.register(fs)
	if err := fs.Parse(append([]string{"-C", dir, "-pass-stats"}, args...)); err != nil {
		t.Fatal(err)
	}
	o.irSpill = spill
	var edges []string
	o.afterRead = func(l *loaded, r *model.Rules, st *sites.Result, g *sg.Program, _ map[string]string) error {
		if spill {
			if l.disk == nil {
				t.Error("the IR is not in the file")
			} else if s := l.disk.Stats(); s.Store.Funcs == 0 || s.Reads == 0 {
				t.Errorf("the file was not used: %+v", s)
			}
		}
		var err error
		edges, err = acceptEdges(filepath.Join(t.TempDir(), "graph.db"), g, st)
		return err
	}
	rep, err := o.analyze(nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	m["edges"] = edges
	return acceptItemsOf(t, m)
}

func irspillSameResult(t *testing.T, dir string, args ...string) {
	t.Helper()
	mem := irspillLangRun(t, false, dir, args...)
	disk := irspillLangRun(t, true, dir, args...)
	for _, k := range acceptItems {
		if mem.items[k] != disk.items[k] {
			t.Errorf("%s differs between the memory and the file:\nmemory %.400s\nfile   %.400s", k, mem.items[k], disk.items[k])
		}
	}
	if len(mem.items["edges"]) < 5 || len(mem.items["findings"]) < 5 {
		t.Errorf("the sample gave nothing to compare: %.100s %.100s", mem.items["edges"], mem.items["findings"])
	}
}

func TestIRSpillDartSameResult(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	ensureDartSamplePub(t, dart)
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	sample := filepath.Join(extractor, "test", "sample")
	irspillSameResult(t, sample, "-lang", "dart", "-dart-extractor", dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"),
		"-rules", filepath.Join(sample, "rules.yaml"))
}

// irspillRustSample copies the sample crate into a temporary directory, with the crate it depends on inside it: the
// sandbox reads the project and nothing beside it.
func irspillRustSample(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	base, err := filepath.Abs(filepath.Join("..", "..", "extractor", "rust", "test"))
	if err != nil {
		t.Fatal(err)
	}
	sample := filepath.Join(root, "sample")
	for _, c := range [][2]string{{"sample", sample}, {"dep", filepath.Join(sample, "dep")}} {
		if out, err := exec.Command("cp", "-R", filepath.Join(base, c[0]), c[1]).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
	}
	m := filepath.Join(sample, "Cargo.toml")
	b, err := os.ReadFile(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m, bytes.Replace(b, []byte(`"../dep"`), []byte(`"dep"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	dm := filepath.Join(sample, "dep", "Cargo.toml")
	d, err := os.ReadFile(dm)
	if err != nil {
		t.Fatal(err)
	}
	// (one workspace: the one of the sample)
	if err := os.WriteFile(dm, bytes.Replace(d, []byte("[workspace]"), nil, 1), 0o644); err != nil {
		t.Fatal(err)
	}
	return sample
}

// The Rust extractor runs under the sandbox of macOS (the sample has no dependency to download).
func TestIRSpillRustSameResult(t *testing.T) {
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	sample := irspillRustSample(t)
	irspillSameResult(t, sample, "-lang", "rust", "-rust-extractor", bin, "-rules", filepath.Join(sample, "rules.yaml"))
}

// A function of the IR of Dart or of Rust that cannot be read from the file stops the run with exit code 1, as the
// one of Go does (TestIRSpillReadFailureStopsTheRun).
func TestIRSpillReadFailureDart(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	ensureDartSamplePub(t, dart)
	extractor, _ := filepath.Abs("../../extractor/dart")
	sample := filepath.Join(extractor, "test", "sample")
	irspillReadFailure(t, sample, []string{"RIVUS_DART_EXTRACTOR=" + dart + " run " + filepath.Join(extractor, "bin", "rivus_dart.dart")},
		"-lang", "dart", "-rules", filepath.Join(sample, "rules.yaml"))
}

func TestIRSpillReadFailureRust(t *testing.T) {
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, _ = filepath.Abs(bin)
	sample := irspillRustSample(t)
	irspillReadFailure(t, sample, nil, "-lang", "rust", "-rust-extractor", bin, "-rules", filepath.Join(sample, "rules.yaml"))
}

func irspillReadFailure(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	run := func(extra ...string) (int, string) {
		cmd := exec.Command(binary, append([]string{"analyze", "-C", dir}, args...)...)
		cmd.Env = append(append(cmd.Environ(), env...), extra...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), stderr.String()
		} else if err != nil {
			t.Fatal(err)
		}
		return 0, stderr.String()
	}
	if code, out := run(envIRSpill + "=1"); code != 0 && code != 3 && code != 4 {
		if strings.Contains(out, "are not fetched") {
			t.Skip("the dependencies of the sample are not fetched on this machine")
		}
		t.Fatalf("the run without a failure: exit %d\n%s", code, out)
	}
	for _, n := range []string{"1", "50"} {
		code, out := run(envIRSpill+"=1", envIRSpillFault+"="+n)
		if code != 1 {
			t.Errorf("the read %s fails: exit %d, want 1\n%s", n, code, out)
		}
		if !strings.Contains(out, "rivus: internal error: the IR of ") || strings.Contains(out, "goroutine ") {
			t.Errorf("the read %s fails: the message is not the internal error:\n%s", n, out)
		}
	}
}
