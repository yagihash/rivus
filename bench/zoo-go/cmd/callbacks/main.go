//go:build zooexec

// Command callbacks writes what the standard library does with the functions it is given (see package callbacks).
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"zoo/callbacks"
)

func main() {
	b, err := json.Marshal(callbacks.Run())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
}
