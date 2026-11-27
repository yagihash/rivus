package builtin_test

import (
	"go/importer"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model/builtin"
)

// The rows of go-callbacks.yaml that say `params` say what the signatures of the standard library say (go/types),
// and the twelve functions whose callbacks the advice of CallWarnings knows (#243) all say it.
func TestCallbackParamsKnown(t *testing.T) {
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	wantRows := map[string]bool{
		"(*sync.Once).Do/arg0": true, "sort.Slice/arg1": true, "sort.SliceStable/arg1": true, "sort.Search/arg1": true,
		"slices.SortFunc/arg1": true, "slices.SortStableFunc/arg1": true, "slices.IndexFunc/arg1": true,
		"slices.ContainsFunc/arg1": true, "strings.IndexFunc/arg1": true, "strings.Map/arg0": true,
		"strings.TrimFunc/arg1": true, "strings.FieldsFunc/arg1": true,
	}
	imp := importer.ForCompiler(token.NewFileSet(), "source", nil)
	seen := map[string]bool{}
	for _, rs := range sets {
		for _, p := range rs.Propagators {
			for _, c := range p.Calls {
				if c.Params == nil {
					continue
				}
				key := p.Symbol + "/" + c.Fn
				seen[key] = true
				var sig *types.Signature
				if strings.HasPrefix(p.Symbol, "(*") {
					pkgName, rest, _ := strings.Cut(strings.TrimPrefix(p.Symbol, "(*"), ".")
					typ, method, _ := strings.Cut(rest, ").")
					pkg, err := imp.Import(pkgName)
					if err != nil {
						t.Fatal(err)
					}
					named := pkg.Scope().Lookup(typ).Type()
					obj, _, _ := types.LookupFieldOrMethod(types.NewPointer(named), true, pkg, method)
					sig = obj.(*types.Func).Type().(*types.Signature)
				} else {
					pkgName, name, _ := strings.Cut(p.Symbol, ".")
					pkg, err := imp.Import(pkgName)
					if err != nil {
						t.Fatal(err)
					}
					sig = pkg.Scope().Lookup(name).Type().(*types.Signature)
				}
				idx := int(c.Fn[len(c.Fn)-1] - '0')
				cb, ok := sig.Params().At(idx).Type().Underlying().(*types.Signature)
				if !ok {
					t.Errorf("%s: parameter %d is not a function", key, idx)
					continue
				}
				if cb.Params().Len() != *c.Params {
					t.Errorf("%s: the callback has %d parameters, params says %d", key, cb.Params().Len(), *c.Params)
				}
			}
		}
	}
	for key := range wantRows {
		if !seen[key] {
			t.Errorf("%s: the row does not say params", key)
		}
	}
}
