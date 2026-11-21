package main

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// GoUniverse is the type information of the Go standard library (and of the packages of the module the
// tool is run in): what a symbol of the model names, and what shape a function has by its types.
type GoUniverse struct {
	Pkgs map[string]*types.Package
	Std  []string // the import paths of the public packages of the standard library
}

func LoadGoUniverse(dir string, extra []string) (*GoUniverse, error) {
	u := &GoUniverse{Pkgs: map[string]*types.Package{}}
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps, Dir: dir}
	std, err := packages.Load(cfg, "std")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, p := range std {
		u.visit(p, seen)
		if isPublicStd(p.PkgPath) {
			u.Std = append(u.Std, p.PkgPath)
		}
	}
	sort.Strings(u.Std)
	// the other packages the model names (grpc, protobuf, x/...): the ones the module of this tool can load
	if len(extra) > 0 {
		pk, _ := packages.Load(cfg, extra...)
		for _, p := range pk {
			u.visit(p, seen)
		}
	}
	return u, nil
}

// visit registers the types of a package and of what it imports. A package with errors is left out: its scope
// may be partial, and "missing" must mean that the name is not there.
func (u *GoUniverse) visit(p *packages.Package, seen map[string]bool) {
	if seen[p.PkgPath] {
		return
	}
	seen[p.PkgPath] = true
	if p.Types != nil && len(p.Errors) == 0 {
		u.Pkgs[p.PkgPath] = p.Types
	}
	for _, d := range p.Imports {
		u.visit(d, seen)
	}
}

func isPublicStd(path string) bool {
	if strings.HasPrefix(path, "vendor/") || strings.HasPrefix(path, "cmd/") || path == "unsafe" || path == "C" {
		return false
	}
	return !strings.Contains(path, "internal") && !strings.HasPrefix(path, "testing/") || path == "testing"
}

// Resolve looks a go/types FullName up. status: "found", "missing" (the package is loaded and the name is not
// in it), "unloaded" (the package is not available here, or the name is a generic instance).
func (u *GoUniverse) Resolve(sym string) (*types.Func, string) {
	if strings.ContainsAny(sym, "[]{}") {
		return nil, "unloaded"
	}
	if strings.HasPrefix(sym, "(") {
		i := strings.Index(sym, ")")
		if i < 0 || i+1 >= len(sym) || sym[i+1] != '.' {
			return nil, "unloaded"
		}
		inner, meth := strings.TrimPrefix(sym[1:i], "*"), sym[i+2:]
		j := strings.LastIndex(inner, ".")
		if j < 0 {
			return nil, "unloaded"
		}
		pkg := u.Pkgs[inner[:j]]
		if pkg == nil {
			return nil, "unloaded"
		}
		obj := pkg.Scope().Lookup(inner[j+1:])
		tn, ok := obj.(*types.TypeName)
		if !ok {
			return nil, "missing"
		}
		recvT := tn.Type()
		if _, isIface := recvT.Underlying().(*types.Interface); !isIface {
			recvT = types.NewPointer(recvT)
		}
		o, _, _ := types.LookupFieldOrMethod(recvT, true, pkg, meth)
		if fn, ok := o.(*types.Func); ok {
			return fn, "found"
		}
		return nil, "missing"
	}
	j := strings.LastIndex(sym, ".")
	if j < 0 {
		return nil, "unloaded"
	}
	pkg := u.Pkgs[sym[:j]]
	if pkg == nil {
		return nil, "unloaded"
	}
	if fn, ok := pkg.Scope().Lookup(sym[j+1:]).(*types.Func); ok {
		return fn, "found"
	}
	return nil, "missing"
}

// carrier says whether a value of the type can hold text or data that came from outside: strings, byte
// slices, readers and writers, empty interfaces, and the containers and structs of those.
func carrier(t types.Type, depth int) bool {
	if depth > 3 || t == nil {
		return false
	}
	switch t := t.(type) {
	case *types.Basic:
		return t.Info()&types.IsString != 0
	case *types.TypeParam:
		return true
	case *types.Named:
		if t.Obj().Pkg() == nil && t.Obj().Name() == "error" {
			return false
		}
		return carrier(t.Underlying(), depth+1)
	case *types.Pointer:
		return carrier(t.Elem(), depth+1)
	case *types.Slice:
		if b, ok := t.Elem().Underlying().(*types.Basic); ok && b.Kind() == types.Byte {
			return true
		}
		return carrier(t.Elem(), depth+1)
	case *types.Array:
		return carrier(t.Elem(), depth+1)
	case *types.Chan:
		return carrier(t.Elem(), depth+1)
	case *types.Map:
		return carrier(t.Key(), depth+1) || carrier(t.Elem(), depth+1)
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if carrier(t.Field(i).Type(), depth+1) {
				return true
			}
		}
		return false
	case *types.Interface:
		if t.NumMethods() == 0 {
			return true
		}
		return ioLike(t)
	}
	return false
}

func ioLike(t *types.Interface) bool {
	for i := 0; i < t.NumMethods(); i++ {
		switch t.Method(i).Name() {
		case "Read", "Write", "WriteString", "ReadFrom", "WriteTo", "ReadString", "String":
			return true
		}
	}
	return false
}

func isFuncType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Signature)
	return ok
}

// writerLike: a value the callee can write to by calling a method of it (io.Writer) or that is any value
// (json.Unmarshal(data, &v) is given an any).
func writable(t types.Type) (strong, weak bool) {
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		return carrier(u.Elem(), 1), false
	case *types.Interface:
		if u.NumMethods() == 0 {
			return true, false
		}
		for i := 0; i < u.NumMethods(); i++ {
			switch u.Method(i).Name() {
			case "Write", "WriteString", "WriteByte", "WriteRune":
				return true, false
			}
		}
		return false, false
	case *types.Slice, *types.Map:
		return false, carrier(t, 1)
	}
	if _, ok := t.(*types.TypeParam); ok {
		return true, false
	}
	return false, false
}

// FuncShape is the shape of a function by its types. "" when it carries nothing (no parameter or receiver that can
// hold data). The second result is true when the writing is only possible (a slice or a map that may be a destination).
func FuncShape(fn *types.Func) (shape string, weak bool) {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return "", false
	}
	var ins []types.Type
	if r := sig.Recv(); r != nil {
		ins = append(ins, r.Type())
	}
	for i := 0; i < sig.Params().Len(); i++ {
		ins = append(ins, sig.Params().At(i).Type())
	}
	carries, callback, strong, weakW := false, false, false, false
	for i, t := range ins {
		if isFuncType(t) {
			callback = true
			continue
		}
		if carrier(t, 0) {
			carries = true
		}
		s, w := writable(t)
		if i == 0 && sig.Recv() != nil {
			// a pointer receiver can be written to by the method; an interface receiver is the reader or writer itself
			if _, isPtr := t.Underlying().(*types.Pointer); isPtr && carrier(t, 0) {
				s = true
			} else {
				s = false
			}
			if iface, isI := t.Underlying().(*types.Interface); isI && ioLike(iface) {
				for k := 0; k < iface.NumMethods(); k++ {
					switch iface.Method(k).Name() {
					case "Write", "WriteString":
						s = true
					}
				}
			}
		}
		strong = strong || s
		weakW = weakW || w
	}
	if !carries && !callback {
		return "", false
	}
	switch {
	case callback:
		return ShapeCall, false
	case strong:
		return ShapeWrite, false
	case weakW && !resultCarries(sig):
		// a slice or a map is the destination when the function does not hand data back (io.ReadFull, sort.Strings, copy)
		return ShapeWrite, true
	case sig.Results().Len() > 0:
		return ShapeValue, false
	}
	return ShapeNone, false
}

// GoAPI is one public function or method of the standard library that can carry data.
type GoAPI struct {
	Symbol string `json:"symbol"`
	Shape  string `json:"shape"`
	Weak   bool   `json:"weak,omitempty"`
}

// Enumerate lists the public API of the standard library that has a parameter, receiver or result that can carry
// data: no denominator is made by hand.
func (u *GoUniverse) Enumerate() []GoAPI {
	var out []GoAPI
	add := func(fn *types.Func) {
		if !fn.Exported() {
			return
		}
		sh, weak := FuncShape(fn)
		if sh == "" {
			return
		}
		out = append(out, GoAPI{Symbol: fn.FullName(), Shape: sh, Weak: weak})
	}
	for _, path := range u.Std {
		pkg := u.Pkgs[path]
		if pkg == nil {
			continue
		}
		sc := pkg.Scope()
		for _, name := range sc.Names() {
			obj := sc.Lookup(name)
			if !obj.Exported() {
				continue
			}
			switch o := obj.(type) {
			case *types.Func:
				add(o)
			case *types.TypeName:
				if o.IsAlias() {
					continue
				}
				named, ok := o.Type().(*types.Named)
				if !ok {
					continue
				}
				if named.TypeParams().Len() > 0 {
					continue // the symbol of a generic type has its parameters in it
				}
				for i := 0; i < named.NumMethods(); i++ {
					add(named.Method(i))
				}
				if it, ok := named.Underlying().(*types.Interface); ok {
					for i := 0; i < it.NumExplicitMethods(); i++ {
						add(it.ExplicitMethod(i))
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

func (a GoAPI) String() string { return fmt.Sprintf("%s [%s]", a.Symbol, a.Shape) }

// resultCarries says whether a result of the function can hold data (not an int, a bool or an error).
func resultCarries(sig *types.Signature) bool {
	for i := 0; i < sig.Results().Len(); i++ {
		if carrier(sig.Results().At(i).Type(), 0) {
			return true
		}
	}
	return false
}
