// Package zoosynth stands for an external package of the zoo (docs/DESIGN-package-models.md, 6.8): a skeleton in a module of its
// own, so that the cells of bench/zoo-go-pkg/zoo_synth call a function of another module. It is not a package of anyone.
package zoosynth

// Echo returns what it was given.
func Echo(s string) string { return s }
