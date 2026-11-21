// Package exec_output_echo is a cell of the zoo.
//
// @cell(F/conv/exec-output-echo)
// @shape: a program prints its argument (echo), and what the program wrote to its output is the sink: the argument (set in cmd.Args, so that the constructor is not a sink of it) reaches the output of the process; the model of Output says it may, as an approximation, with an uncertainty
package exec_output_echo

import (
	"os/exec"

	"zoo/mark"
)

func init() {
	mark.Case("F/conv/exec-output-echo/vuln", vuln)
	mark.Case("F/conv/exec-output-echo/fixed", fixed)
	mark.Case("F/conv/exec-output-echo/decoy", decoy)
}

func vuln() { // @vuln
	cmd := exec.Command("echo")
	cmd.Args = append(cmd.Args, mark.Src("conv-exec-output-echo-vuln")) // @source(conv-exec-output-echo-vuln)
	out, _ := cmd.Output()
	mark.SinkHTML(string(out)) // @sink(conv-exec-output-echo-vuln)
}

func fixed() { // @fixed
	cmd := exec.Command("echo")
	cmd.Args = append(cmd.Args, mark.CleanHTML(mark.Src("conv-exec-output-echo-fixed")))
	out, _ := cmd.Output()
	mark.SinkHTML(string(out)) // @sink(none)
}

func decoy() { // @decoy
	a := exec.Command("echo")
	a.Args = append(a.Args, mark.Src("conv-exec-output-echo-decoy")) // @source(conv-exec-output-echo-decoy)
	b := exec.Command("echo", "constant")
	outA, _ := a.Output()
	out, _ := b.Output()
	mark.SinkHTML(string(out)) // @sink(none) @nosink(conv-exec-output-echo-decoy)
	mark.SinkSQL(string(outA)) // @sink(conv-exec-output-echo-decoy)
}
