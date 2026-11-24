//go:build coarseforce

package main

import (
	"fmt"
	"os"
	"strconv"
)

// forcedCoarseRound is for the tests, and is built only with the tag coarseforce (the binary that is shipped does not
// have it): RIVUS_INTERNAL_SCC_COARSEN_AT_ROUND=n moves the large components from their n-th round (below 0: before the
// first), whatever the counts are (ADR-124).
func forcedCoarseRound() (int, error) {
	v := os.Getenv("RIVUS_INTERNAL_SCC_COARSEN_AT_ROUND")
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("RIVUS_INTERNAL_SCC_COARSEN_AT_ROUND: want a number (got %q)", v)
	}
	return n, nil
}
