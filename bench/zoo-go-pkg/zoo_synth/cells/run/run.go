// Package run is a cell of the package zoo_synth.
//
// @cell(F/pkg/zoo-synth/run)
// @shape: a value of a source goes to a function of a module of another package (a skeleton) that the rules have as a sink; the skeleton records it
package run

import (
	zoosynth "example.invalid/zoosynth"
	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/zoo-synth/run/vuln", vuln)
	mark.Case("F/pkg/zoo-synth/run/fixed", fixed)
	mark.Case("F/pkg/zoo-synth/run/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("pkg-zoo-synth-run-vuln") // @source(pkg-zoo-synth-run-vuln)
	zoosynth.Run(v)                         // @sink(pkg-zoo-synth-run-vuln)
}

func fixed() { // @fixed
	zoosynth.Run(mark.CleanCmd(mark.Src("pkg-zoo-synth-run-fixed"))) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("pkg-zoo-synth-run-decoy") // @source(pkg-zoo-synth-run-decoy)
	zoosynth.Run("constant")                 // @sink(none) @nosink(pkg-zoo-synth-run-decoy)
	zoosynth.Run(t)                          // @sink(pkg-zoo-synth-run-decoy)
}
