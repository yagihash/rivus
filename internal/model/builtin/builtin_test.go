package builtin_test

import (
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

func TestBuiltinRulesAreValid(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) == 0 {
		t.Fatal("no built-in rule sets")
	}
	r := model.Merge(sets...)
	for _, sym := range []string{"os/exec.Command", "(*database/sql.DB).Query", "(net/http.ResponseWriter).Write"} {
		if len(r.Sinks[sym]) == 0 {
			t.Errorf("built-in sink %s missing", sym)
		}
	}
	if len(r.ParamTypes) == 0 {
		t.Error("built-in param_type source missing")
	}
}
