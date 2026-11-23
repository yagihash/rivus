package accept

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/yagihash/rivus/internal/ir"
)

// DiffAll says every place where two functions differ, as paths with the two values. It compares what a record
// keeps, by the same rule as irspill.Diff: every field except the ones with `json:"-"`, and a nil slice is the
// same as an empty one. Unlike irspill.Diff, it does not stop at the first difference.
func DiffAll(a, b *ir.Func) []string {
	return diffValue(reflect.ValueOf(a), reflect.ValueOf(b), "Func", nil)
}

func diffValue(a, b reflect.Value, path string, out []string) []string {
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				out = append(out, path+" (nil on one side)")
			}
			return out
		}
		return diffValue(a.Elem(), b.Elem(), path, out)
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				out = append(out, path+" (nil on one side)")
			}
			return out
		}
		if a.Elem().Type() != b.Elem().Type() {
			return append(out, fmt.Sprintf("%s (type %s and %s)", path, a.Elem().Type(), b.Elem().Type()))
		}
		return diffValue(a.Elem(), b.Elem(), path, out)
	case reflect.Struct:
		t := a.Type()
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Tag.Get("json") == "-" {
				continue
			}
			out = diffValue(a.Field(i), b.Field(i), path+"."+t.Field(i).Name, out)
		}
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return append(out, fmt.Sprintf("%s (length %d and %d)", path, a.Len(), b.Len()))
		}
		for i := 0; i < a.Len(); i++ {
			out = diffValue(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i), out)
		}
	case reflect.Map:
		if a.Len() != b.Len() {
			out = append(out, fmt.Sprintf("%s (length %d and %d)", path, a.Len(), b.Len()))
		}
		keys := a.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
		for _, k := range keys {
			bv := b.MapIndex(k)
			if !bv.IsValid() {
				out = append(out, fmt.Sprintf("%s[%v] (only on one side)", path, k))
				continue
			}
			out = diffValue(a.MapIndex(k), bv, fmt.Sprintf("%s[%v]", path, k), out)
		}
	case reflect.String:
		if a.String() != b.String() {
			out = append(out, fmt.Sprintf("%s (%q and %q)", path, a.String(), b.String()))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			out = append(out, fmt.Sprintf("%s (%d and %d)", path, a.Int(), b.Int()))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if a.Uint() != b.Uint() {
			out = append(out, fmt.Sprintf("%s (%d and %d)", path, a.Uint(), b.Uint()))
		}
	case reflect.Float32, reflect.Float64:
		if a.Float() != b.Float() {
			out = append(out, fmt.Sprintf("%s (%v and %v)", path, a.Float(), b.Float()))
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			out = append(out, fmt.Sprintf("%s (%t and %t)", path, a.Bool(), b.Bool()))
		}
	default:
		out = append(out, path+" (kind "+a.Kind().String()+" not compared)")
	}
	return out
}
