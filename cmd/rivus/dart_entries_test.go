package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/yagihash/rivus/internal/taint"
)

// A Flutter-shaped app (bench/zoo-dart/app): main hands a widget to runApp, and the framework calls
// the methods of the widget (build, initState, dispose) and the closure of a button. The code that was
// read does not call them, but they override what a dependency declares, so the finding says
// from_framework_callback, and not not_from_entry. A method that overrides a method of the app's own
// class, and a function that nothing calls, stay not_from_entry (ADR-073). The comment of each sink in
// the app says what its finding must have.
func TestDartFrameworkCallbacks(t *testing.T) {
	if os.Getenv("RIVUS_DART") == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "zoo-dart", "app"))
	if err != nil {
		t.Fatal(err)
	}
	// what the comments say, by the line of the sink
	want := map[int]string{}
	f, err := os.Open(filepath.Join(dir, "lib", "main.dart"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	re := regexp.MustCompile(`sinkCmd\(.*// (from_entry|from_framework_callback|not_from_entry)`)
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		if m := re.FindStringSubmatch(sc.Text()); m != nil {
			want[n] = m[1]
		}
	}
	if len(want) != 8 {
		t.Fatalf("the app has %d sinks with an expectation, want 8", len(want))
	}

	stream := zooDartStream(t, dir)
	irFile := filepath.Join(t.TempDir(), "app.jsonl")
	if err := os.WriteFile(irFile, stream, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "analyze", "-ir", irFile, "-rules", filepath.Join(dir, "rules.yaml"))
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var res struct {
		Findings []struct {
			Sink struct {
				Position struct{ Line int }
			}
			Reachability     string
			ReachabilityNote string `json:"reachability_note"`
		}
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatal(err)
	}
	got := map[int]string{}
	for _, fd := range res.Findings {
		line := fd.Sink.Position.Line
		got[line] = fd.Reachability
		// the note goes with the two values that are not from_entry, and says which
		switch fd.Reachability {
		case taint.ReachFromEntry:
			if fd.ReachabilityNote != "" {
				t.Errorf("line %d: from_entry has a note", line)
			}
		case taint.ReachFromFrameworkCallback:
			if fd.ReachabilityNote != taint.FrameworkCallbackNote {
				t.Errorf("line %d: the note of from_framework_callback is %q", line, fd.ReachabilityNote)
			}
		case taint.ReachNotFromEntry:
			if fd.ReachabilityNote != taint.NotFromEntryNote {
				t.Errorf("line %d: the note of not_from_entry is %q", line, fd.ReachabilityNote)
			}
		}
	}
	for line, w := range want {
		if got[line] != w {
			t.Errorf("lib/main.dart:%d: reachability %q, want %q", line, got[line], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("%d findings, want %d: %v", len(got), len(want), got)
	}
}
