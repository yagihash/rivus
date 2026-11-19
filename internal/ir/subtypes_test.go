package ir

import (
	"bytes"
	"reflect"
	"testing"
)

func TestSubtypeMethods(t *testing.T) {
	p := &Program{
		Types: []TypeInfo{
			{ID: "g#Service"},
			{ID: "app#Base", Supers: []TypeID{"g#Service"}},
			{ID: "app#Impl", Supers: []TypeID{"app#Base"}},
			{ID: "app#Other"},
		},
		Funcs: []*Func{
			// a handler of a subtype of a subtype: the request is the input
			{ID: "a", Symbol: "app#Impl.run", Recv: true, Params: []Param{{Type: "app#Impl"}, {Type: "g#Call"}, {Type: "app#Req"}}},
			// takes the call and nothing else: nothing to be the input
			{ID: "b", Symbol: "app#Impl.meta", Recv: true, Params: []Param{{Type: "app#Impl"}, {Type: "g#Call"}}},
			// no call parameter: not a handler
			{ID: "c", Symbol: "app#Impl.help", Recv: true, Params: []Param{{Type: "app#Impl"}, {Type: "app#Req"}}},
			// not a subtype
			{ID: "d", Symbol: "app#Other.run", Recv: true, Params: []Param{{Type: "app#Other"}, {Type: "g#Call"}, {Type: "app#Req"}}},
			// a dependency is not the code being analysed
			{ID: "e", Symbol: "app#Impl.dep", Recv: true, Dep: true, Params: []Param{{Type: "app#Impl"}, {Type: "g#Call"}, {Type: "app#Req"}}},
		},
	}
	got := p.SubtypeMethods("g#Service", "g#Call")
	want := []MarkedMethod{{Symbol: "app#Impl.run", Params: []int{1}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !p.IsSubtype("app#Impl", "g#Service") || p.IsSubtype("app#Other", "g#Service") {
		t.Error("IsSubtype is wrong")
	}
}

func TestTypesRoundTrip(t *testing.T) {
	p := &Program{Types: []TypeInfo{{ID: "a#A", Supers: []TypeID{"a#B"}}, {ID: "a#B"}}}
	var buf bytes.Buffer
	if err := Write(&buf, p); err != nil {
		t.Fatal(err)
	}
	q, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Types, q.Types) {
		t.Errorf("got %+v, want %+v", q.Types, p.Types)
	}
}
