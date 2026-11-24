// Package accept is the test that an extractor of Go code gives the same result as the one now in use (goir):
// the same functions, the same IR of each, the same candidates of each call. There is no list of allowed
// differences. One difference fails the test.
package accept

import (
	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
)

// Options are what the analysis passes to the extractor.
type Options struct {
	Tests, NoDeps bool
	DepDirs       []string
	// ReplacedPackage and ReplacedFunc are the packages and functions the models stand for.
	ReplacedPackage func(pkg string) bool
	ReplacedFunc    func(id string) bool
}

// Extractor reads the Go code under dir and makes the IR of it the way the analysis needs it: every function
// with a body (the dependencies included), the candidates of the calls, and the texts.
type Extractor struct {
	Name string
	Run  func(dir string, patterns []string, o Options) (*ir.Program, error)
}

// Current is the extractor in use: gossa.Load and goir.Extract, called with the arguments of the analysis
// (cmd/rivus, common.load and loaded.extract with the graph).
var Current = Extractor{Name: "goir", Run: func(dir string, patterns []string, o Options) (*ir.Program, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	p, err := goir.Load(dir, patterns, gossa.Options{
		Tests: o.Tests, Deps: !o.NoDeps, DepDirs: o.DepDirs,
		ReplacedPackage: o.ReplacedPackage, ReplacedFunc: o.ReplacedFunc,
	})
	if err != nil {
		return nil, err
	}
	prog, err := goir.Extract(p, goir.Options{All: true})
	if err != nil {
		return nil, err
	}
	goir.AddTexts(goir.NewTexts(p), prog)
	return prog, nil
}}
