//go:build !coarseforce

package main

// forcedCoarseRound: the shipped binary does not read a forced round (coarse_force.go, tag coarseforce).
func forcedCoarseRound() (int, error) { return 0, nil }
