// Package receiver_field_across_entries is a cell of the zoo.
//
// @cell(F/state/receiver-field-across-entries)
// @shape: one entry point (a handler) writes a field of the receiver from its parameter, and another entry point of the same receiver reads the field (R-58)
package receiver_field_across_entries

import "zoo/mark"

func init() {
	mark.Case("F/state/receiver-field-across-entries/vuln", vuln)
	mark.Case("F/state/receiver-field-across-entries/fixed", fixed)
	mark.Case("F/state/receiver-field-across-entries/decoy", decoy)
}

type server struct{ cache string }

func (s *server) handleA(in mark.In) { // @source(state-receiver-field-across-entries-vuln)
	s.cache = in.Value()
}

func (s *server) handleB() {
	mark.SinkCmd(s.cache) // @sink(state-receiver-field-across-entries-vuln)
}

func vuln() { // @vuln
	s := &server{}
	s.handleA(mark.NewIn("state-receiver-field-across-entries-vuln"))
	s.handleB()
}

type serverFixed struct{ cache string }

func (s *serverFixed) handleA(in mark.In) {
	s.cache = in.Value()
}

func (s *serverFixed) handleB() {
	mark.SinkCmd(mark.CleanCmd(s.cache)) // @sink(none)
}

func fixed() { // @fixed
	s := &serverFixed{}
	s.handleA(mark.NewIn("state-receiver-field-across-entries-fixed"))
	s.handleB()
}

type serverDecoy struct{ cache, other string }

func (s *serverDecoy) handleA(in mark.In) { // @source(state-receiver-field-across-entries-decoy)
	s.cache = in.Value()
}

func (s *serverDecoy) handleB() {
	mark.SinkCmd(s.other) // @sink(none) @nosink(state-receiver-field-across-entries-decoy)
	mark.SinkSQL(s.cache) // @sink(state-receiver-field-across-entries-decoy)
}

func decoy() { // @decoy
	s := &serverDecoy{other: "constant"}
	s.handleA(mark.NewIn("state-receiver-field-across-entries-decoy"))
	s.handleB()
}
