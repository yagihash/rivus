package report

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/taint"
)

// The stub names the arguments a function can write into (from the types), and leaves the context out of the writes.
func TestStubNamesWritableArguments(t *testing.T) {
	u := taint.NewUncertainty(taint.KindUnmodeledLibraryCall, "", "lib.Fill")
	u.Recv = &taint.Operand{Type: "*lib.T", Class: "pointer"}
	u.Operands = []taint.Operand{
		{Type: "context.Context", Class: "interface"},
		{Type: "[]byte", Class: "container"},
		{Type: "string", Class: "string"},
		{Type: "*lib.Buf", Class: "pointer"},
	}
	r := &Report{Uncertainties: []*taint.Uncertainty{u}}
	got := r.Coverage().Symbols[0].Stub
	for _, want := range []string{
		"{from: args, to: results}",
		"to: arg1}", "to: arg3}", "{from: recv, to: arg1}", "{from: arg1, to: arg3}", "{from: arg3, to: recv}",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stub lacks %q:\n%s", want, got)
		}
	}
	for _, bad := range []string{"to: arg0}", "to: arg2}", "from: arg0,", "from: args, to: args"} {
		if strings.Contains(got, bad) {
			t.Errorf("stub has %q:\n%s", bad, got)
		}
	}
}
