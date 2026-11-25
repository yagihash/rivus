// extractor-hash prints the hash of the source of an extractor: `go run ./scripts/extractor-hash rust|dart [dir]` (dir is extractor/<lang> by
// default). `make build` and scripts/build-release-archives.sh give it to the build of rivus (-ldflags -X), and `rivus setup` computes the same
// value for the build of the extractor (internal/extractorhash, ADR-126).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/yagihash/rivus/internal/extractorhash"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(stderr, "usage: extractor-hash rust|dart [dir]")
		return 2
	}
	dir := filepath.Join("extractor", args[0])
	if len(args) == 2 {
		dir = args[1]
	}
	h, err := extractorhash.Compute(args[0], dir)
	if err != nil {
		fmt.Fprintln(stderr, "extractor-hash:", err)
		return 1
	}
	fmt.Fprintln(stdout, h)
	return 0
}
