package main

import (
	"flag"
	"testing"
)

// A graph file is an input that cannot be trusted: the flags it records are taken only when flagValues is what writes them.
// Before, any flag that the command knew was set from the record: -dart-extractor, -rust-extractor, -unsafe-host-exec and -isolation-read too.
func TestAdoptStoredFlagsOnlyTheRecordedOnes(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	k := fs.Int("k", 1, "")
	tests := fs.Bool("tests", false, "")
	dart := fs.String("dart-extractor", "", "")
	rust := fs.String("rust-extractor", "", "")
	unsafe := fs.Bool("unsafe-host-exec", false, "")
	var read multiFlag
	fs.Var(&read, "isolation-read", "")
	adoptStoredFlags(fs, map[string]string{
		"k": "5", "tests": "true",
		"dart-extractor": "/tmp/evil", "rust-extractor": "/tmp/evil", "unsafe-host-exec": "true", "isolation-read": "/",
	})
	if *k != 5 || !*tests {
		t.Errorf("the flags that the file records are taken: k=%d tests=%v", *k, *tests)
	}
	if *dart != "" || *rust != "" || *unsafe || len(read) != 0 {
		t.Errorf("flags that flagValues does not write were taken from the file: dart=%q rust=%q unsafe=%v read=%v", *dart, *rust, *unsafe, read)
	}
}

// Every key that flagValues can write is one that is taken (or one that adoptStoredFlags handles on its own): a new key is not lost.
func TestStoredFlagNamesCoverFlagValues(t *testing.T) {
	o := &analyzeOpts{
		lang: "go", tests: true, noDeps: true, numeric: false, charTaint: true, noBuiltinSS: true, readDeclared: true,
		rules: multiFlag{"r.yaml"}, bodyReplaced: multiFlag{"p"}, depDirs: multiFlag{"d"}, allowDepsUnder: multiFlag{"a"},
	}
	own := map[string]bool{"patterns": true, "allow-deps-under": true, "dep-dir": true}
	for name := range o.flagValues([]string{"./..."}) {
		if !storedFlagNames[name] && !own[name] {
			t.Errorf("flagValues writes %q, which storedFlagNames does not have: a question with -C would not take it", name)
		}
	}
}
