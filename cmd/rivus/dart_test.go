package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// The Dart extractor writes the neutral IR of a sample package; rivus reads it
// and finds the flows that the annotations of the sample say it must find, and
// no other. It needs the Dart SDK: RIVUS_DART names the command that runs it,
// for example "dart" or "mise exec dart@3.13.6 -- dart".
func TestDartExtractor(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		t.Skip("RIVUS_DART is not set")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	args := append(words[1:], "run", "bin/rivus_dart.dart", "test/sample")
	cmd := exec.Command(words[0], args...)
	cmd.Dir = extractor
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v\n%s", err, stderr.String())
	}
	irFile := filepath.Join(t.TempDir(), "sample.jsonl")
	if err := os.WriteFile(irFile, stream, 0o644); err != nil {
		t.Fatal(err)
	}

	rules := filepath.Join(extractor, "test", "sample", "rules.yaml")
	run := exec.Command(binary, "analyze", "-ir", irFile, "-rules", rules, "-no-builtin-rules")
	out, err := run.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}

	// The annotations: @source(tag) and @sink(tag) in lib/app.dart.
	annot := regexp.MustCompile(`//\s*@(source|sink)\(([A-Za-z0-9_-]+)\)`)
	src, sinks := map[string]int{}, map[string]int{}
	var none []int
	f, err := os.Open(filepath.Join(extractor, "test", "sample", "lib", "app.dart"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		if m := annot.FindStringSubmatch(sc.Text()); m != nil {
			switch {
			case m[1] == "source":
				src[m[2]] = n
			case m[2] == "none":
				none = append(none, n)
			default:
				sinks[m[2]] = n
			}
		}
	}
	want := map[string]bool{}
	for tag, s := range src {
		want[fmt.Sprintf("%d -> %d", s, sinks[tag])] = true
	}
	got := map[string]bool{}
	for _, fd := range rep.Findings {
		got[fmt.Sprintf("%d -> %d", fd.Source.Position.Line, fd.Sink.Position.Line)] = true
		for _, n := range none {
			if fd.Sink.Position.Line == n {
				t.Errorf("a flow reaches the sink at line %d, which must stay quiet", n)
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
	if len(want) < 10 {
		t.Errorf("the sample has only %d flows", len(want))
	}
}
