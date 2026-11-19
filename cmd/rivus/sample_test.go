package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// checkSampleFlows analyses the IR that an extractor wrote for a sample and
// compares what rivus finds with the annotations in the sample's files:
// "@source(tag)" and "@sink(tag)" on a line say that the data of the source
// reaches the sink; "@sink(none)" says that nothing reaches it. The files are
// named relative to the root of the sample, as the positions of the stream
// are. min is the least number of flows the sample has.
func checkSampleFlows(t *testing.T, stream []byte, rules, root string, files []string, min int) {
	t.Helper()
	irFile := filepath.Join(t.TempDir(), "sample.jsonl")
	if err := os.WriteFile(irFile, stream, 0o644); err != nil {
		t.Fatal(err)
	}
	// The built-in models are in force; the rules of the sample only name its
	// own source, sink and sanitizer.
	out, err := exec.Command(binary, "analyze", "-ir", irFile, "-rules", rules).Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}

	annot := regexp.MustCompile(`//\s*@(source|sink)\(([A-Za-z0-9_-]+)\)`)
	type at struct {
		file string
		line int
	}
	src, sinks := map[string]at{}, map[string]at{}
	var none []at
	for _, rel := range files {
		f, err := os.Open(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			if m := annot.FindStringSubmatch(sc.Text()); m != nil {
				here := at{rel, n}
				switch {
				case m[1] == "source":
					src[m[2]] = here
				case m[2] == "none":
					none = append(none, here)
				default:
					sinks[m[2]] = here
				}
			}
		}
		f.Close()
	}
	want := map[string]bool{}
	for tag, s := range src {
		want[fmt.Sprintf("%s:%d -> %s:%d", s.file, s.line, sinks[tag].file, sinks[tag].line)] = true
	}
	got := map[string]bool{}
	for _, fd := range rep.Findings {
		got[fmt.Sprintf("%s:%d -> %s:%d", fd.Source.Position.File, fd.Source.Position.Line, fd.Sink.Position.File, fd.Sink.Position.Line)] = true
		// the samples are libraries: their exported functions are where they start, whatever
		// initializers of variables they hold (yagihash/rivus #60)
		if fd.Reachability == "not_from_entry" {
			t.Errorf("a flow at %s:%d is not from an entry", fd.Sink.Position.File, fd.Sink.Position.Line)
		}
		for _, n := range none {
			if fd.Sink.Position.File == n.file && fd.Sink.Position.Line == n.line {
				t.Errorf("a flow reaches the sink at %s:%d, which must stay quiet", n.file, n.line)
			}
		}
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing flow %s", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("unexpected flow %s", k)
		}
	}
	if len(want) < min {
		t.Errorf("the sample has only %d flows", len(want))
	}
}
