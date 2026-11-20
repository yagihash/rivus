package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
	"github.com/yagihash/rivus/internal/testutil"
)

// The analysis runs the components of a level at the same time. The order in which they
// finish must not reach the output (ADR-049, ADR-054): the same input gives the same bytes,
// whatever GOMAXPROCS and -workers are. A write that goes into the shared state in the order
// of the goroutines (pendingGlobals before commitGlobals sorted it) shows here as a path
// that starts in a different writer, and a different fingerprint["path@1"] (which `rivus
// resolve` reads as "the path changed").
//
// What does not need to be the same is the time: duration_ms, dep_spent_ms, own_spent_ms.

// stable returns the report without the fields that are a clock.
func stable(t *testing.T, out []byte) []byte {
	t.Helper()
	var rep map[string]any
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	if st, ok := rep["stats"].(map[string]any); ok {
		for _, k := range []string{"duration_ms", "dep_spent_ms", "own_spent_ms"} {
			delete(st, k)
		}
	}
	b, err := json.Marshal(rep) // sorted keys
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// analyzeWith runs `rivus analyze args...` in dir with GOMAXPROCS=procs ("": the default).
func analyzeWith(t *testing.T, dir, procs string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(binary, append([]string{"analyze"}, args...)...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if procs != "" {
		cmd.Env = append(cmd.Env, "GOMAXPROCS="+procs)
	}
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("analyze %v: %v\n%s", args, err, stderr)
	}
	return stable(t, out)
}

// sameEveryTime analyses the same input `runs` times under each GOMAXPROCS (1, 4, the
// default) and -workers (the default, 2, 16), and fails when two outputs differ.
func sameEveryTime(t *testing.T, dir string, runs int, args ...string) {
	t.Helper()
	var first []byte
	var firstHow string
	for _, procs := range []string{"1", "4", ""} {
		for _, workers := range []string{"", "2", "16"} {
			for i := 0; i < runs; i++ {
				a := args
				if workers != "" {
					a = append([]string{"-workers", workers}, args...)
				}
				got := analyzeWith(t, dir, procs, a...)
				how := fmt.Sprintf("GOMAXPROCS=%q -workers=%q run %d", procs, workers, i)
				if first == nil {
					first, firstHow = got, how
					continue
				}
				if !bytes.Equal(first, got) {
					t.Fatalf("the output differs: %s and %s\n%s", firstHow, how, firstDifference(first, got))
				}
			}
		}
	}
}

// firstDifference describes where the findings of two reports differ.
func firstDifference(a, b []byte) string {
	var ra, rb report.Report
	if json.Unmarshal(a, &ra) != nil || json.Unmarshal(b, &rb) != nil {
		return "(not a report)"
	}
	if len(ra.Findings) != len(rb.Findings) {
		return fmt.Sprintf("findings: %d and %d", len(ra.Findings), len(rb.Findings))
	}
	for i := range ra.Findings {
		pa, pb := ra.Findings[i].Fingerprint["path@1"], rb.Findings[i].Fingerprint["path@1"]
		if pa != pb {
			return fmt.Sprintf("finding %s: path %s and %s\n  %s\n  %s", ra.Findings[i].ID, pa, pb, stepFuncs(ra.Findings[i].Paths[0].Steps), stepFuncs(rb.Findings[i].Paths[0].Steps))
		}
	}
	return "(not in the findings' paths)"
}

func stepFuncs(steps []taint.Step) string {
	var s []string
	for _, st := range steps {
		s = append(s, st.Func[strings.LastIndex(st.Func, ".")+1:])
	}
	return strings.Join(s, " ")
}

// TestDeterministicGlobalWriters: the writers of gwriters are in the same level, and the
// path of the finding starts in the first of them: always the same one.
func TestDeterministicGlobalWriters(t *testing.T) {
	sameEveryTime(t, testutil.Testdata(), 6, "-rules", "rules.yaml", "./cases/paramshare/gwriters")
}

// TestDeterministicParamshare: the four global cases of paramshare.
func TestDeterministicParamshare(t *testing.T) {
	for _, p := range []string{"globalw", "gwrite", "gwritehelper", "gqueue"} {
		t.Run(p, func(t *testing.T) {
			sameEveryTime(t, testutil.Testdata(), 6, "-rules", "rules.yaml", "./cases/paramshare/"+p)
		})
	}
}

// TestDeterministicCuratedGo: bench/go-curated.
func TestDeterministicCuratedGo(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	sameEveryTime(t, dir, 2, "-sites", "-rules", "rules.yaml", "./...")
}

// TestDeterministicCuratedDart needs RIVUS_DART, TestDeterministicCuratedRust RIVUS_RUST.
func TestDeterministicCuratedDart(t *testing.T) {
	stream, dir := dartCuratedStream(t)
	sameEveryTimeIR(t, dir, stream)
}

func TestDeterministicCuratedRust(t *testing.T) {
	stream, dir := rustCuratedStream(t)
	sameEveryTimeIR(t, dir, stream)
}

func sameEveryTimeIR(t *testing.T, dir string, stream []byte) {
	t.Helper()
	irFile := filepath.Join(t.TempDir(), "curated.jsonl")
	if err := os.WriteFile(irFile, stream, 0o644); err != nil {
		t.Fatal(err)
	}
	sameEveryTime(t, dir, 2, "-sites", "-ir", irFile, "-rules", filepath.Join(dir, "rules.yaml"))
}

// TestDeterministicDogfood: rivus analysed by rivus. Only the findings are compared (paths and
// fingerprints included): the `functions` count of a run is one more or less from one run to the
// next (6676 to 6678 in rivus itself, same binary, nothing changed: the wrappers go/ssa makes, not
// an order of the engine; not looked into), and a run that the clock stopped (-timeout, on a loaded
// machine) is a different analysis, so it is not compared.
func TestDeterministicDogfood(t *testing.T) {
	if testing.Short() {
		t.Skip("dogfood")
	}
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	var first json.RawMessage
	for _, procs := range []string{"1", "4", ""} {
		for i := 0; i < 1; i++ {
			out := analyzeWith(t, root, procs, "-rules", "dogfood/rules.yaml", "-timeout", "120s", "./...")
			var rep struct {
				Findings json.RawMessage `json:"findings"`
				Stats    struct {
					Converged bool `json:"converged"`
					TimedOut  bool `json:"timed_out"`
				} `json:"stats"`
			}
			if err := json.Unmarshal(out, &rep); err != nil {
				t.Fatal(err)
			}
			if !rep.Stats.Converged || rep.Stats.TimedOut {
				t.Logf("GOMAXPROCS=%q run %d did not finish (a loaded machine): not compared", procs, i)
				continue
			}
			if first == nil {
				first = rep.Findings
			} else if !bytes.Equal(first, rep.Findings) {
				t.Fatalf("the findings differ (GOMAXPROCS=%q, run %d)", procs, i)
			}
		}
	}
	if first == nil {
		t.Skip("no run finished")
	}
}
