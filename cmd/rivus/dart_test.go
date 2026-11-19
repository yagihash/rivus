package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
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
	checkSampleFlows(t, stream, filepath.Join(extractor, "test", "sample", "rules.yaml"),
		filepath.Join(extractor, "test", "sample"),
		[]string{"lib/app.dart", "lib/web.dart", "lib/server.dart", "lib/patterns.dart", "lib/callbacks.dart", "lib/syntax.dart", "lib/grpc_flows.dart", "lib/review_try.dart", "lib/review_ctor.dart", "lib/review_misc.dart", "lib/gen_model.dart", "lib/gen_model.g.dart", "lib/review_issues.dart", "lib/src/build/command.dart"}, 73)

	// What the extractor could not express is said in the IR, and the sample has nothing of the kind
	// (REVIEW-2026-11-19: nothing is left out without a trace).
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range prog.Funcs {
		if f.Dep {
			continue
		}
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpUnsupported {
					t.Errorf("%s: unsupported: %s", f.ID, in.Reason)
				}
			}
		}
	}
}
