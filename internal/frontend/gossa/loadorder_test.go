package gossa

import (
	"go/token"
	"testing"
)

// The switch of the order of the load (loadOrderEnv) does change the order: the packages come in the reverse order, and
// the files are in the file set at other places. If it changed nothing, the tests that use it would see nothing.
func TestLoadOrderSwitchChangesTheOrder(t *testing.T) {
	load := func() ([]string, map[string]int) {
		p, err := Load(testdata, []string{"./..."}, Options{NaiveForm: true})
		if err != nil {
			t.Fatal(err)
		}
		bases := map[string]int{}
		p.Fset.Iterate(func(f *token.File) bool { bases[f.Name()] = f.Base(); return true })
		return p.PkgPaths, bases
	}
	paths, bases := load()
	t.Setenv(loadOrderEnv, "a")
	paths2, bases2 := load()
	if len(paths) < 2 || paths[0] != paths2[len(paths2)-1] {
		t.Fatalf("the packages are not in the reverse order: %v and %v", paths, paths2)
	}
	moved := 0
	for n, b := range bases {
		if bases2[n] != b {
			moved++
		}
	}
	if moved == 0 {
		t.Fatal("no file went into the file set at another place")
	}
}
