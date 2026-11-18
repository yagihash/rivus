// Package lib provides the sources, sinks and sanitizers used by the test
// cases. The functions are modeled by testdata/go/rules.yaml; the bodies are
// never analysed because lib is not part of the loaded patterns.
package lib

// Source returns attacker controlled data.
func Source() string { return "tainted" }

// SourceBytes returns attacker controlled bytes.
func SourceBytes() []byte { return []byte("tainted") }

// Sink must not receive attacker controlled data.
func Sink(s string) {}

// SinkBytes must not receive attacker controlled data.
func SinkBytes(b []byte) {}

// SinkAny must not receive attacker controlled data.
func SinkAny(v any) {}

// Sanitize removes the taint.
func Sanitize(s string) string { return s }

// Pair is a struct with two fields.
type Pair struct {
	A string
	B string
}

// Runner is an interface used for dispatch tests.
type Runner interface {
	Run(s string)
}

// Fill writes attacker controlled data through the pointer, as a database read
// does (rows.Scan(&x)).
func Fill(dst *string) error { return nil }

// FillAny is Fill for a destination given as any (db.Find(&rows)).
func FillAny(dst any) error { return nil }

// StoreRead reads from the data store; nothing is followed through it.
func StoreRead() string { return "" }
