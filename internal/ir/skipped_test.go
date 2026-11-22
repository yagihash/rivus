package ir

import (
	"bytes"
	"reflect"
	"testing"
)

func TestFnSkippedRoundTripsThroughTheStream(t *testing.T) {
	p := &Program{FnSkipped: []FnSkipped{
		{Unit: "google_cloud_auth", Func: "google_cloud_auth::x::{closure#0}", Dependency: true, Why: "not implemented: FIXME(async_closures)"},
		{Unit: "mine", Func: "mine::f", Why: "a panic"},
	}}
	var buf bytes.Buffer
	if err := Write(&buf, p); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.FnSkipped, p.FnSkipped) {
		t.Errorf("got %+v, want %+v", got.FnSkipped, p.FnSkipped)
	}
}
