// Package echo is a cell of the package zoo_synth.
//
// @cell(F/pkg/zoo-synth/echo)
// @shape: a value of a source goes through a function of a module of another package (a skeleton) to a sink
package echo

import (
	zoosynth "example.invalid/zoosynth"
	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/zoo-synth/echo/vuln", vuln)
	mark.Case("F/pkg/zoo-synth/echo/fixed", fixed)
	mark.Case("F/pkg/zoo-synth/echo/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("pkg-zoo-synth-echo-vuln") // @source(pkg-zoo-synth-echo-vuln)
	mark.SinkCmd(zoosynth.Echo(v))           // @sink(pkg-zoo-synth-echo-vuln)
}

func fixed() { // @fixed
	v := mark.CleanCmd(zoosynth.Echo(mark.Src("pkg-zoo-synth-echo-fixed")))
	mark.SinkCmd(v) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("pkg-zoo-synth-echo-decoy") // @source(pkg-zoo-synth-echo-decoy)
	mark.SinkCmd(zoosynth.Echo("constant"))   // @sink(none) @nosink(pkg-zoo-synth-echo-decoy)
	mark.SinkCmd(zoosynth.Echo(t))            // @sink(pkg-zoo-synth-echo-decoy)
}
