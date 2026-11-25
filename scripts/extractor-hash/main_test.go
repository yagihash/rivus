package main

import (
	"bytes"
	"testing"

	"github.com/yagihash/rivus/internal/extractorhash"
)

func TestPrintsTheHashOfTheTree(t *testing.T) {
	for _, lang := range extractorhash.Langs() {
		want, err := extractorhash.Compute(lang, "../../extractor/"+lang)
		if err != nil {
			t.Fatal(err)
		}
		var out, errb bytes.Buffer
		if code := run([]string{lang, "../../extractor/" + lang}, &out, &errb); code != 0 || out.String() != want+"\n" {
			t.Errorf("%s: code %d, out %q, err %q, want %s", lang, code, out.String(), errb.String(), want)
		}
	}
}

func TestUsageAndErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 2 {
		t.Errorf("no argument: code %d, want 2", code)
	}
	if code := run([]string{"go"}, &out, &errb); code != 1 {
		t.Errorf("a language without an extractor: code %d, want 1", code)
	}
	if code := run([]string{"rust", t.TempDir()}, &out, &errb); code != 1 {
		t.Errorf("a directory that is not an extractor: code %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("printed %q on a failure", out.String())
	}
}
