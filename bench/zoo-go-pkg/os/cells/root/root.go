// Package root is a cell of the package os.
//
// @cell(F/pkg/os/root)
// @shape: a name given to an os.Root is not a sink (the root is what the name cannot leave), and what it makes (the error that names it) goes on to the sink
package root

import (
	"os"

	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/os/root/vuln", vuln)
	mark.Case("F/pkg/os/root/fixed", fixed)
	mark.Case("F/pkg/os/root/decoy", decoy)
}

func vuln() { // @vuln
	root, err := os.OpenRoot(os.TempDir())
	if err != nil {
		return
	}
	defer root.Close()
	name := mark.Src("pkg-os-root-vuln") // @source(pkg-os-root-vuln)
	// the name goes to the methods that read, and none of them is a sink (the rows have the flow, not a sink)
	if f, err := root.Open(name); err == nil {
		f.Close()
	}
	_, _ = os.Lstat(name) // a row of flows like os.Stat, not a sink
	_, _ = root.Stat(name)
	_, _ = root.Lstat(name)
	_, _ = root.Readlink(name)
	_, err = root.ReadFile(name)
	mark.SinkHTML(err.Error()) // @sink(pkg-os-root-vuln)
}

func fixed() { // @fixed
	root, err := os.OpenRoot(os.TempDir())
	if err != nil {
		return
	}
	defer root.Close()
	name := mark.CleanHTML(mark.Src("pkg-os-root-fixed"))
	_, err = root.ReadFile(name)
	mark.SinkHTML(err.Error()) // @sink(none)
}

func decoy() { // @decoy
	root, err := os.OpenRoot(os.TempDir())
	if err != nil {
		return
	}
	defer root.Close()
	name := mark.Src("pkg-os-root-decoy") // @source(pkg-os-root-decoy)
	_, errA := root.ReadFile(name)
	_, errB := root.ReadFile("zoo-no-such-file")
	mark.SinkHTML(errB.Error()) // @sink(none) @nosink(pkg-os-root-decoy)
	mark.SinkHTML(errA.Error()) // @sink(pkg-os-root-decoy)
}
