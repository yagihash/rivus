package main

import (
	"os"
	"strings"
	"testing"
)

// skipOrFail is for a test that needs an extractor (RIVUS_DART, RIVUS_RUST). It skips when the variable is not
// set, unless the run says that the extractor is required (RIVUS_REQUIRE_DART=1, RIVUS_REQUIRE_RUST=1: the
// jobs of CI that are about it set it): then a variable that went missing fails the test, and does not turn
// the whole suite into a green run of skipped tests.
func skipOrFail(t *testing.T, name string) {
	t.Helper()
	require := "RIVUS_REQUIRE_" + strings.TrimPrefix(name, "RIVUS_")
	if os.Getenv(require) == "1" {
		t.Fatalf("%s is not set, and %s=1 says that the test must run", name, require)
	}
	t.Skip(name + " is not set")
}
