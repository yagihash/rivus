package gossa

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Symbol returns the rule symbol of a function: the go/types FullName of the
// declared function or method, e.g. "(*net/http.Request).FormValue". It is
// empty for anonymous and synthetic functions.
func Symbol(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	if o := fn.Origin(); o != nil {
		fn = o
	}
	if f, ok := fn.Object().(*types.Func); ok && f != nil {
		return f.FullName()
	}
	return ""
}

// CalleeSymbol returns the rule symbol of the called function of a call: the
// static callee, or the interface method for an invoke-mode call. It is empty
// for calls through function values and builtins.
func CalleeSymbol(call *ssa.CallCommon) string {
	if call.IsInvoke() {
		return call.Method.FullName()
	}
	return Symbol(call.StaticCallee())
}

// CallArgs splits a call into its receiver (nil for plain functions and
// function values) and its arguments, receiver excluded.
func CallArgs(call *ssa.CallCommon) (recv ssa.Value, args []ssa.Value) {
	if call.IsInvoke() {
		return call.Value, call.Args
	}
	if fn := call.StaticCallee(); fn != nil && fn.Signature.Recv() != nil && len(call.Args) > 0 {
		return call.Args[0], call.Args[1:]
	}
	return nil, call.Args
}

// IsConstant reports whether v is a compile-time constant, possibly wrapped
// in conversions, or a slice literal made only of constants (as produced for
// variadic arguments).
func IsConstant(v ssa.Value) bool {
	return isConstant(v, 0)
}

func isConstant(v ssa.Value, depth int) bool {
	if depth > 8 {
		return false
	}
	switch v := v.(type) {
	case *ssa.Const:
		return true
	case *ssa.MakeInterface:
		return isConstant(v.X, depth+1)
	case *ssa.ChangeType:
		return isConstant(v.X, depth+1)
	case *ssa.Convert:
		return isConstant(v.X, depth+1)
	case *ssa.BinOp:
		return isConstant(v.X, depth+1) && isConstant(v.Y, depth+1)
	case *ssa.Slice:
		alloc, ok := v.X.(*ssa.Alloc)
		if !ok {
			return false
		}
		for _, r := range *alloc.Referrers() {
			switch r := r.(type) {
			case *ssa.Slice:
			case *ssa.IndexAddr:
				for _, rr := range *r.Referrers() {
					st, ok := rr.(*ssa.Store)
					if !ok || st.Addr != ssa.Value(r) || !isConstant(st.Val, depth+1) {
						return false
					}
				}
			default:
				return false
			}
		}
		return true
	}
	return false
}
