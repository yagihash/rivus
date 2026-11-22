// Package zoosynth stands for an external package of the zoo (docs/DESIGN-package-models.md, 6.8): a skeleton in a module of its
// own, so that the cells of bench/zoo-go-pkg/zoo_synth call a function of another module. It is not a package of anyone.
// A function that the model of the package has as a sink records what it was given with zoo/mark (the run of the cell checks the
// row: the place of the record is the call in the cell).
package zoosynth

import "zoo/mark"

// Echo returns what it was given.
func Echo(s string) string { return s }

// Run stands for a function with a sink row: it records its argument.
func Run(cmd string) { mark.SinkCmd(cmd) }
