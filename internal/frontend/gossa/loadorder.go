package gossa

import (
	"go/ast"
	"go/parser"
	"go/token"
	"hash/fnv"
	"os"
	"time"

	"golang.org/x/tools/go/packages"
)

// loadOrderEnv is for the tests of the extractor only (internal/frontend/accept): when it is set, the load is made in
// another order than it is by itself, and the result must be the same. It is not a flag, it is not in the help, and
// the analysis never sets it. The value is a seed (any text).
//
// What it changes: the patterns and the packages are taken in the reverse order, and each file waits a time that
// depends on the seed and the name before it is parsed, so the files go into the file set in another order
// (the token.Pos of every place changes; the parse is parallel, so this is an order a run can have by itself).
const loadOrderEnv = "RIVUS_INTERNAL_LOAD_ORDER"

// shakeOrder changes the order of the load as loadOrderEnv says; it does nothing when it is not set.
func shakeOrder(cfg *packages.Config, patterns []string) []string {
	seed := os.Getenv(loadOrderEnv)
	if seed == "" {
		return patterns
	}
	cfg.ParseFile = func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
		h := fnv.New32a()
		h.Write([]byte(seed + "\x00" + filename))
		time.Sleep(time.Duration(h.Sum32()%7) * time.Millisecond)
		return parser.ParseFile(fset, filename, src, parser.AllErrors|parser.ParseComments)
	}
	out := append([]string(nil), patterns...)
	reverse(out)
	return out
}

// shakePackages reverses the order of the packages the load gave, when loadOrderEnv is set.
func shakePackages(pkgs []*packages.Package) {
	if os.Getenv(loadOrderEnv) != "" {
		reverse(pkgs)
	}
}

func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
