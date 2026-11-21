//go:build zooexec

// Command pure writes what the pure rows of the built-in model give for the marker (see package pure).
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"zoo/pure"
)

func main() {
	b, err := json.Marshal(pure.Run())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
}
