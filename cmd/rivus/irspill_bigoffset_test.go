package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The Rust extractor writes the offset of a position as line * 100000 + column, so that the offset of a line that is
// beyond 21474 is more than 2^31, and one beyond 86000 is more than 4 * 2^31. The IR in the file keeps such an offset
// (the record keeps all 64 bits of a position), and the result is the one of the IR in memory.

// bigOffsets adds shift to every offset of a stream: the offset of a position, the offsets of a text, and the offset
// of a text of a call.
func bigOffsets(t *testing.T, stream []byte, shift float64) []byte {
	t.Helper()
	var walk func(v any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				if n, ok := e.(float64); ok && k == "offset" {
					x[k] = n + shift
					continue
				}
				if a, ok := e.([]any); ok && k == "offsets" {
					for i := range a {
						if n, ok := a[i].(float64); ok {
							a[i] = n + shift
						}
					}
					continue
				}
				x[k] = walk(e)
			}
		case []any:
			for i := range x {
				x[i] = walk(x[i])
			}
		}
		return v
	}
	var out bytes.Buffer
	for _, line := range bytes.Split(bytes.TrimSpace(stream), []byte("\n")) {
		var v any
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.UseNumber()
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		// numbers as float64: the offsets of the sample are small, and the shift is a whole number below 2^53
		b, _ := json.Marshal(v)
		var w any
		if err := json.Unmarshal(b, &w); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(walk(w))
		if err != nil {
			t.Fatal(err)
		}
		out.Write(b)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

func TestIRSpillBigOffsetsSameResult(t *testing.T) {
	stream, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ir", "dart_exceptions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := filepath.Abs(filepath.Join("..", "..", "testdata", "ir", "dart_exceptions_rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		shift float64
	}{
		{"none", 0},
		{"line 90000", 100_000 * 90_000},
		{"line 4 million", 100_000 * 4_000_000},
	} {
		t.Run(c.name, func(t *testing.T) {
			irFile := filepath.Join(t.TempDir(), "big.jsonl")
			if err := os.WriteFile(irFile, bigOffsets(t, stream, c.shift), 0o644); err != nil {
				t.Fatal(err)
			}
			irspillSameResult(t, t.TempDir(), "-ir", irFile, "-rules", rules)
		})
	}
}

// A constant of Go that does not fit 32 bits is in the IR as text, and it is not a number the record keeps.
func TestIRSpillGoBigConstantSameResult(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/big\n\ngo 1.21\n",
		"main.go": `package main

import "time"

const big = 1 << 40
const huge uint64 = 1<<64 - 1

func main() {
	d := time.Second * 10
	_ = big + int64(d)
	_ = huge
	sink(source())
}

func source() string { return "x" }

func sink(s string) {}
`,
	}
	for n, c := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rules := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(rules, []byte("ruleset: test/big@1\nsources:\n  - symbol: example.com/big.source\n    labels: [user_input]\nsinks:\n  - symbol: example.com/big.sink\n    category: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	acceptSpillBoth(t, dir, []string{"-rules", rules}, "./...")
}
