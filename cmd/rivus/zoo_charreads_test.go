package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// charReadsOf analyses one program (an IR stream) and gives summary.numeric_stops.char_reads.total.
func charReadsOf(t *testing.T, dir string, stream []byte) (int, []report.NumericStopCount) {
	t.Helper()
	irFile := filepath.Join(t.TempDir(), "cell.jsonl")
	if err := os.WriteFile(irFile, stream, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "analyze", "-sites", "-ir", irFile, "-rules", filepath.Join(dir, "rules.yaml"))
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	n := rep.Summary.NumericStops.CharReads
	return n.Total, n.Top
}

// TestZooCharReadsDart holds summary.numeric_stops.char_reads to the cells F/conv/char-* of the Dart zoo (#185): a
// shape that is missed because a code unit or a rune is read as a number has a count above 0 (a count of 0 there would
// say "nothing stopped" where something did), and a shape that is found has none.
func TestZooCharReadsDart(t *testing.T) {
	root := zooDartRoot(t)
	_, cells := zooDartScan(t, root)
	found := map[string]bool{"F/conv/char-reverse": true, "F/conv/char-index": true, "F/conv/char-rune-slice": true}
	// the element that reaches a callback (s.runes.map((r) => ...)) is a parameter of the closure: it is not counted (#185)
	notCounted := map[string]bool{"F/conv/char-replace": true}
	n := 0
	for _, c := range cells {
		if !strings.HasPrefix(c.name, "F/conv/char-") {
			continue
		}
		n++
		dir := zooDartCopy(t, root, []*zooCell{c})
		total, top := charReadsOf(t, dir, zooDartStream(t, dir))
		t.Logf("%s: char_reads %d %v", c.name, total, top)
		if notCounted[c.name] {
			if total != 0 {
				t.Errorf("%s: counted now: take it out of notCounted, and out of the notes in docs", c.name)
			}
			continue
		}
		if found[c.name] && total != 0 {
			t.Errorf("%s: a found shape has char_reads %d", c.name, total)
		}
		if !found[c.name] && total == 0 {
			t.Errorf("%s: a missed shape has char_reads 0", c.name)
		}
	}
	if n != 10 {
		t.Errorf("%d char cells, want 10", n)
	}
}

// TestZooCharReadsRust is TestZooCharReadsDart for the Rust zoo. Run with RIVUS_RUST (extractor/rust: cargo build).
func TestZooCharReadsRust(t *testing.T) {
	root := zooRustRoot(t)
	_, cells := zooRustScan(t, root)
	found := map[string]bool{"F/conv/char-replace": true, "F/conv/char-reverse": true, "F/conv/char-index": true, "F/conv/char-rune-slice": true}
	// found, and counted all the same: the closure of chars().map(|c| ...) takes a char that is compared (c == 'S'): a
	// count says "a flow may have stopped at a char read here", not "a flow was lost"
	notCounted := map[string]bool{"F/conv/char-replace": true}
	for _, c := range cells {
		if !strings.HasPrefix(c.name, "F/conv/char-") {
			continue
		}
		dir := zooRustCopyCells(t, root, []*zooCell{c})
		total, top := charReadsOf(t, dir, zooRustStream(t, dir))
		t.Logf("%s: char_reads %d %v", c.name, total, top)
		if notCounted[c.name] {
			continue // not checked either way
		}
		if found[c.name] && total != 0 {
			t.Errorf("%s: a found shape has char_reads %d", c.name, total)
		}
		if !found[c.name] && total == 0 {
			t.Errorf("%s: a missed shape has char_reads 0", c.name)
		}
	}
}
