package irspill

import (
	"fmt"
	"reflect"

	"github.com/yagihash/rivus/internal/ir"
)

// Diff says where two functions differ, as the first path it finds ("" when they are the same). It compares what a
// record keeps: every field except the ones with `json:"-"`, and a nil slice is the same as an empty one. It is
// for the tests of the record, and for the check of a program that went through a store.
func Diff(a, b *ir.Func) string {
	return diff(reflect.ValueOf(a), reflect.ValueOf(b), "Func")
}

func diff(a, b reflect.Value, path string) string {
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				return path + " (nil)"
			}
			return ""
		}
		return diff(a.Elem(), b.Elem(), path)
	case reflect.Struct:
		t := a.Type()
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Tag.Get("json") == "-" {
				continue
			}
			if d := diff(a.Field(i), b.Field(i), path+"."+t.Field(i).Name); d != "" {
				return d
			}
		}
	case reflect.Slice:
		if a.Len() != b.Len() {
			return fmt.Sprintf("%s (length %d and %d)", path, a.Len(), b.Len())
		}
		for i := 0; i < a.Len(); i++ {
			if d := diff(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i)); d != "" {
				return d
			}
		}
	case reflect.String:
		if a.String() != b.String() {
			return fmt.Sprintf("%s (%q and %q)", path, a.String(), b.String())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			return fmt.Sprintf("%s (%d and %d)", path, a.Int(), b.Int())
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			return path + " (bool)"
		}
	default:
		return path + " (kind " + a.Kind().String() + " not compared)"
	}
	return ""
}
