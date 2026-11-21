// Package save_restore is a cell of the zoo.
//
// @cell(F/state/save-restore)
// @shape: a function saves a field, sets it, calls the work and restores it; the decoy reads another field
package save_restore

import "zoo/mark"

func init() {
	mark.Case("F/state/save-restore/vuln", vuln)
	mark.Case("F/state/save-restore/fixed", fixed)
	mark.Case("F/state/save-restore/decoy", decoy)
}

type state struct{ a, b string }

var sVuln state

func readVuln() { mark.SinkCmd(sVuln.a) } // @sink(state-save-restore-vuln)

func vuln() { // @vuln
	old := sVuln.a
	sVuln.a = mark.Src("state-save-restore-vuln") // @source(state-save-restore-vuln)
	readVuln()
	sVuln.a = old
}

var sFixed state

func readFixed() { mark.SinkCmd(sFixed.a) } // @sink(none)

func fixed() { // @fixed
	old := sFixed.a
	sFixed.a = mark.CleanCmd(mark.Src("state-save-restore-fixed"))
	readFixed()
	sFixed.a = old
}

var sDecoy state

func readDecoyB() { mark.SinkCmd(sDecoy.b) } // @sink(none) @nosink(state-save-restore-decoy)
func readDecoyA() { mark.SinkSQL(sDecoy.a) } // @sink(state-save-restore-decoy)

func decoy() { // @decoy
	old := sDecoy.a
	sDecoy.a = mark.Src("state-save-restore-decoy") // @source(state-save-restore-decoy)
	sDecoy.b = "constant"
	readDecoyB()
	readDecoyA()
	sDecoy.a = old
}
