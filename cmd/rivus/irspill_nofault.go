//go:build !irspillfault

package main

// spillFault is the failure that is put in a read of the IR on disk, for the tests (ADR-123). The binary that is
// shipped has none: irspill_fault.go is built only with the tag irspillfault.
func spillFault() func(read int) error { return nil }
