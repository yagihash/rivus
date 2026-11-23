package report

import (
	"reflect"
	"testing"

	"github.com/yagihash/rivus/internal/taint"
)

func TestCategoryFilter(t *testing.T) {
	fs := []*taint.Finding{
		{ID: "1", Category: "a"},
		{ID: "2", Category: "b"},
		{ID: "3", Category: "b"},
		{ID: "4", Category: "a", Categories: []string{"a", "c"}},
	}
	ids := func(l []*taint.Finding) (o []string) {
		for _, f := range l {
			o = append(o, f.ID)
		}
		return
	}
	var none *CategoryFilter
	if k, e := none.Apply(fs); len(k) != 4 || e != nil {
		t.Errorf("nil filter: %v %v", ids(k), e)
	}
	if k, e := (&CategoryFilter{}).Apply(fs); len(k) != 4 || e != nil {
		t.Errorf("empty filter: %v %v", ids(k), e)
	}
	k, e := (&CategoryFilter{Include: []string{"b"}}).Apply(fs)
	if !reflect.DeepEqual(ids(k), []string{"2", "3"}) || !reflect.DeepEqual(e, []CategoryExcluded{{"a", 2}}) {
		t.Errorf("include b: %v %v", ids(k), e)
	}
	// a finding with several categories stays when one of them is included, and goes only when all are left out
	k, e = (&CategoryFilter{Exclude: []string{"a"}}).Apply(fs)
	if !reflect.DeepEqual(ids(k), []string{"2", "3", "4"}) || !reflect.DeepEqual(e, []CategoryExcluded{{"a", 1}}) {
		t.Errorf("exclude a: %v %v", ids(k), e)
	}
	k, e = (&CategoryFilter{Exclude: []string{"a", "c"}}).Apply(fs)
	if !reflect.DeepEqual(ids(k), []string{"2", "3"}) || !reflect.DeepEqual(e, []CategoryExcluded{{"a", 2}}) {
		t.Errorf("exclude a,c: %v %v", ids(k), e)
	}
}
