package sites

import (
	"testing"

	"github.com/yagihash/rivus/internal/model"
)

func TestUncoveredDropsTheSinkThatTheInterfaceSaysTheSameOf(t *testing.T) {
	own := []model.Sink{{Symbol: "I.m", Args: []int{0}, Category: "ssrf", Accepts: []string{"user_input"}}}
	same := model.Sink{Symbol: "C.m", Args: []int{0}, Category: "ssrf", Accepts: []string{"user_input"}}
	other := model.Sink{Symbol: "C.m", Args: []int{1}, Category: "ssrf", Accepts: []string{"user_input"}}
	got := uncovered([]model.Sink{same, other}, own)
	if len(got) != 1 || got[0].Args[0] != 1 {
		t.Fatalf("uncovered = %+v, want only the sink of another position", got)
	}
	if got := uncovered([]model.Sink{same}, nil); len(got) != 1 {
		t.Fatalf("with no sink on the interface the sink of the implementation stays: %+v", got)
	}
}
