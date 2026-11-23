//go:build irspillfault

package main

import (
	"fmt"
	"os"
	"strconv"
)

// spillFault is for the tests of the failure to read, and is built only with the tag irspillfault (the binary that is
// shipped does not have it): with RIVUS_INTERNAL_IR_SPILL_FAULT=n, every read of a function from the file from the n-th
// on fails.
func spillFault() func(read int) error {
	const env = "RIVUS_INTERNAL_IR_SPILL_FAULT"
	n, err := strconv.Atoi(os.Getenv(env))
	if err != nil || n <= 0 {
		return nil
	}
	return func(read int) error {
		if read >= n {
			return fmt.Errorf("a failure put in by %s=%d", env, n)
		}
		return nil
	}
}
