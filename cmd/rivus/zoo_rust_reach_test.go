package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/yagihash/rivus/internal/taint"
)

// The app of the Rust zoo (bench/zoo-rust-app): a main that builds a server, whose handlers a framework finds through a trait that
// the app implements (the shape of rustypaste and actix-web, docs/issues/234-rust-reachability-label-not-from-entry.md). The cells of
// bench/zoo-rust have no main, so their `from_entry` is the reading of a library and says nothing about a program; here main is there.
//
// The comment of each sink in the app says what its finding must have, as in the Dart app (TestDartFrameworkCallbacks):
//
//	// from_entry                  not not_from_entry: from_entry, or from_framework_callback (whichever way a fix finds the handlers)
//	// from_entry @miss(not_from_entry)     the same expectation, and the engine says not_from_entry today: a ratchet, as every @miss (it
//	                               fails when the finding is not not_from_entry any more: remove the @miss in the same commit)
//	// not_from_entry              the contrast: nothing registers the code, and the label must say so
func TestZooRustReach(t *testing.T) {
	t.Parallel()
	src, err := filepath.Abs(filepath.Join("..", "..", "bench", "zoo-rust-app"))
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		reach string
		miss  bool
	}
	wants := map[string]want{}
	re := regexp.MustCompile(`sink_cmd\(.*// (from_entry|from_framework_callback|not_from_entry)( @miss\(not_from_entry\))?`)
	for _, name := range []string{"main.rs", "server.rs"} {
		f, err := os.Open(filepath.Join(src, "app", "src", name))
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			if m := re.FindStringSubmatch(sc.Text()); m != nil {
				wants[fmt.Sprintf("app/src/%s:%d", name, n)] = want{m[1], m[2] != ""}
			}
		}
		f.Close()
	}
	if len(wants) != 9 {
		t.Fatalf("the app has %d sinks with an expectation, want 9", len(wants))
	}

	// the app reads zoo-mark at ../zoo-rust/zoo-mark: the copy keeps the two directories side by side
	tmp := t.TempDir()
	app := filepath.Join(tmp, "zoo-rust-app")
	mark := filepath.Join(tmp, "zoo-rust", "zoo-mark")
	if err := os.MkdirAll(filepath.Dir(mark), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][2]string{{src, app}, {filepath.Join(zooRustRoot(t), "zoo-mark"), mark}} {
		if out, err := exec.Command("cp", "-R", c[0], c[1]).CombinedOutput(); err != nil {
			t.Fatalf("cp %s: %v\n%s", c[0], err, out)
		}
	}
	os.RemoveAll(filepath.Join(app, "target"))
	os.RemoveAll(filepath.Join(mark, "target"))

	stream := zooRustStream(t, app)
	// the internal marks of the trait of a call and of an impl (traitdef.rs) are removed by the merge of the extractor
	for _, mark := range []string{"impl_trait_def", "by_trait_def"} {
		if bytes.Contains(stream, []byte(mark)) {
			t.Errorf("the stream has the internal mark %s", mark)
		}
	}
	irFile := filepath.Join(t.TempDir(), "app.jsonl")
	if err := os.WriteFile(irFile, stream, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "analyze", "-ir", irFile, "-rules", filepath.Join(src, "rules.yaml"))
	cmd.Dir = src
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var res struct {
		Findings []struct {
			Sink struct {
				Position struct {
					File string
					Line int
				}
			}
			Reachability     string
			ReachabilityNote string `json:"reachability_note"`
		}
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, fd := range res.Findings {
		key := fmt.Sprintf("%s:%d", filepath.ToSlash(fd.Sink.Position.File), fd.Sink.Position.Line)
		got[key] = fd.Reachability
		// the note goes with the two values that are not from_entry, and says which
		switch fd.Reachability {
		case taint.ReachFromEntry:
			if fd.ReachabilityNote != "" {
				t.Errorf("%s: from_entry has a note", key)
			}
		case taint.ReachFromFrameworkCallback:
			if fd.ReachabilityNote != taint.FrameworkCallbackNote {
				t.Errorf("%s: the note of from_framework_callback is %q", key, fd.ReachabilityNote)
			}
		case taint.ReachNotFromEntry:
			if fd.ReachabilityNote != taint.NotFromEntryNote {
				t.Errorf("%s: the note of not_from_entry is %q", key, fd.ReachabilityNote)
			}
		}
	}

	var keys []string
	for k := range wants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	miss, contrast, contrastOK := 0, 0, 0
	for _, k := range keys {
		w, g := wants[k], got[k]
		switch {
		case w.reach == taint.ReachNotFromEntry:
			contrast++
			if g == taint.ReachNotFromEntry {
				contrastOK++
			} else {
				t.Errorf("%s: reachability %q, want not_from_entry (the contrast: nothing registers this code)", k, g)
			}
		case w.miss:
			miss++
			if g != taint.ReachNotFromEntry {
				t.Errorf("%s: reachability %q: the engine finds the handler now, remove @miss(not_from_entry) in the same commit", k, g)
			}
		case g != taint.ReachFromEntry && g != taint.ReachFromFrameworkCallback:
			t.Errorf("%s: reachability %q, want from_entry or from_framework_callback", k, g)
		}
	}
	if len(got) != len(wants) {
		t.Errorf("%d findings, want %d: %v", len(got), len(wants), got)
	}
	state := "ok"
	if contrastOK != contrast {
		state = fmt.Sprintf("FAILED %d / %d", contrast-contrastOK, contrast)
	}
	t.Logf("zoo-rust-app: reach cells %d | miss %d | contrast %s", len(wants), miss, state)
}
