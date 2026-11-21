//go:build !gcpexec

// Package opaque holds functions that the analysis has neither the body nor a model of (a declaration without a
// body: opaque.s makes that legal). They are what a function of a dependency that is not read is to the analysis.
// The harness (exec_test.go) is built with -tags gcpexec, which has the bodies.
package opaque

import "cloud.google.com/go/spanner"

// Move moves the "name" parameter of a statement to its SQL: what a function that is not read may do to a
// structure that it is given.
func Move(s *spanner.Statement)

// Rewrite returns the statement with the "name" parameter moved to its SQL.
func Rewrite(s spanner.Statement) spanner.Statement
