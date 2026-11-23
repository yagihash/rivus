package main

import (
	"github.com/yagihash/rivus/internal/frontend/accept"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/taint"
)

// extractGo makes the IR of the Go code that load read: goir.Extract, or the extractor a test put in (common.extractor).
// That extractor reads the code itself, with the arguments the analysis gives to load, so that two extractors are
// compared on what the analysis passes. The go/ssa program of load stays for what the rules and the report ask of it.
func (c *common) extractGo(l *loaded, patterns []string, rules *model.Rules) error {
	if c.extractor == nil {
		return l.extract(true, c.irSpill)
	}
	o := accept.Options{Tests: c.tests, NoDeps: c.noDeps, DepDirs: c.depDirs}
	if rules.HasDeclared() && !c.dropAfterRead {
		o.ReplacedPackage = rules.ReplacedBody
		o.ReplacedFunc = func(id string) bool { return rules.ReplacedBody(taint.UnitOf(id)) }
	}
	prog, err := c.extractor.Run(c.dir, patterns, o)
	if err != nil {
		return err
	}
	l.ir = prog
	l.texts = ir.NewStreamTexts(prog)
	if c.irSpill {
		l.disk = spillProgram(prog)
		l.src = l.disk
	}
	l.gossa.ReleaseSSA()
	return nil
}
