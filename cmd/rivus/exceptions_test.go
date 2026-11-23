package main

import (
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// A thrown object reaches its fields in a handler that uses the variable as it is (`on T catch (e)`
// with `e.msg`, a dynamic `e.msg`), not only in one that casts it first (yagihash/rivus #70). The IR is
// what the Dart extractor writes for testdata's cases (extractor/dart/test/sample/lib/review_try.dart:
// catchTyped, catchDynamic, catchTypedCallee), so that this runs without the Dart SDK.
func TestThrownObjectFieldsReachTheHandler(t *testing.T) {
	t.Parallel()
	out, err := exec.Command(binary, "analyze", "-ir", "../../testdata/ir/dart_exceptions.jsonl", "-rules", "../../testdata/ir/dart_exceptions_rules.yaml").Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 3 {
		t.Errorf("want 3 findings (typed catch, dynamic catch, typed catch of a callee's throw), got %d", len(rep.Findings))
	}
}
