package main

import (
	"testing"

	"github.com/yagihash/rivus/internal/model"
)

// The check of the rows is one that can fail: a row that says the wrong thing is found.
func TestCallbackCheckFindsAWrongRow(t *testing.T) {
	o := cbObserved{
		Symbol:  "x.Map",
		Markers: map[string]string{"recv": "RECV", "arg1": "ARG1"},
		Calls:   1,
		Params:  []string{"[RECV]"},
		Result:  "(" + cbRet + ")",
	}
	good := model.CallRow{Fn: "arg0", With: []string{"recv"}, Ret: "result0"}
	if msg := cbCheck(good, o); msg != "" {
		t.Errorf("a right row: %s", msg)
	}
	for name, row := range map[string]model.CallRow{
		"wrong with":      {Fn: "arg0", With: []string{"arg1"}, Ret: "result0"},
		"missing with":    {Fn: "arg0", Ret: "result0"},
		"too many with":   {Fn: "arg0", With: []string{"recv", "arg1"}, Ret: "result0"},
		"ret not carried": {Fn: "arg0", With: []string{"recv"}},
	} {
		if msg := cbCheck(row, o); msg == "" {
			t.Errorf("%s: a wrong row was accepted", name)
		}
	}
	o.Result = "()"
	if msg := cbCheck(good, o); msg == "" {
		t.Errorf("ret to a result that does not have what the closure returned was accepted")
	}
	o.Calls = 0
	if msg := cbCheck(good, o); msg == "" {
		t.Errorf("a closure that was not called was accepted")
	}
}
