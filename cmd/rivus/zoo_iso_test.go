package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// zooCopy copies the files of the zoo that one analysis needs (go.mod, rules.yaml, mark, and the given
// cell directories of cells/) to a new directory.
func zooCopy(t *testing.T, dir string, cells []string) string {
	t.Helper()
	dst := t.TempDir()
	for _, f := range []string{"go.mod", "rules.yaml"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range append([]string{"mark"}, cells...) {
		if out, err := exec.Command("cp", "-R", filepath.Join(dir, d), mkdirParent(t, filepath.Join(dst, d))).CombinedOutput(); err != nil {
			t.Fatalf("cp %s: %v\n%s", d, err, out)
		}
	}
	return dst
}

func mkdirParent(t *testing.T, p string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// zooAlone analyses each cell by itself, as a subtest. A cell that fails in the whole zoo (TestZooGo) and
// is green here is mixed up with the others; one that is red here is wrong by itself.
func zooAlone(t *testing.T, dir string, cells []*zooCell) {
	t.Helper()
	for _, c := range cells {
		c := c
		t.Run("alone/"+strings.TrimPrefix(c.name, "F/"), func(t *testing.T) {
			rec := false
			b, _ := os.ReadFile(filepath.Join(dir, c.dir, filepath.Base(c.dir)+".go"))
			rec = strings.Contains(string(b), "@miss(recursion_approximation)")
			one := zooCopy(t, dir, []string{c.dir})
			checkAnnotated(t, one, annotOpts{pattern: "./cells/...", allowUnconverged: rec, programWide: rec})
		})
	}
}

// zooFindings analyses the zoo and returns its findings (source -> sink, reachability, confidence).
func zooFindings(t *testing.T, dir string, env ...string) []string {
	t.Helper()
	cmd := exec.Command(binary, "analyze", "-sites", "-rules", "rules.yaml", "./cells/...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	return zooParseFindings(t, out)
}

// zooParseFindings reads the report of an analysis: its findings as "source -> sink reachability confidence".
func zooParseFindings(t *testing.T, out []byte) []string {
	t.Helper()
	var rep struct {
		Findings []struct {
			Source, Sink struct {
				Position struct {
					File string
					Line int
				}
			}
			Reachability string
			Confidence   string
		}
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	var fs []string
	for _, f := range rep.Findings {
		fs = append(fs, f.Source.Position.File+":"+strconv.Itoa(f.Source.Position.Line)+" -> "+f.Sink.Position.File+":"+strconv.Itoa(f.Sink.Position.Line)+" "+f.Reachability+" "+f.Confidence)
	}
	sort.Strings(fs)
	return fs
}

// TestZooMarkIndependent holds the promise that the analysis reads only the declarations of mark: with the
// real implementation of mark (the recording, which every case writes to one global, and the registration of
// the cases in a table that a harness calls by value) the findings, their reachability and their confidence
// are the same. GOFLAGS=-tags=zooexec makes the loader read mark_exec.go and the harness of the cases too.
func TestZooMarkIndependent(t *testing.T) {
	dir := zooRoot(t)
	plain := zooFindings(t, dir)
	exec := zooFindings(t, dir, "GOFLAGS=-tags=zooexec")
	if strings.Join(plain, "\n") != strings.Join(exec, "\n") {
		t.Errorf("the findings change with the implementation of mark:\n plain: %v\n zooexec: %v", plain, exec)
	}
	if len(plain) == 0 {
		t.Error("no findings")
	}
	t.Logf("zoo-go: %d findings, the same with the implementation of mark and the registration of the cases", len(plain))
}
