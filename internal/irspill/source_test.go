package irspill

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// funcsWithDuplicates makes n random functions, some of which have the ID of an earlier one.
func funcsWithDuplicates(r *rand.Rand, n int) []*ir.Func {
	var out []*ir.Func
	for i := 0; i < n; i++ {
		id := fmt.Sprint("p.F", i)
		if i > 3 && r.Intn(5) == 0 {
			id = string(out[r.Intn(len(out))].ID)
		}
		f := randomFunc(r, id)
		f.Dep, f.Recv = r.Intn(2) == 0, r.Intn(2) == 0
		out = append(out, f)
	}
	return out
}

func diskOf(t *testing.T, funcs []*ir.Func, opt SourceOptions) *DiskSource {
	t.Helper()
	st := NewIn(t.TempDir(), 0, nil)
	b := NewBuilder(st, opt)
	for _, f := range funcs {
		b.Add(f)
	}
	return b.Source(&ir.Program{})
}

func memOf(funcs []*ir.Func) ir.Source {
	return ir.MemSource(&ir.Program{Funcs: append([]*ir.Func(nil), funcs...)})
}

// The Source in the file answers as the Source in memory does: the same functions in the same order, the same heads, the
// last function for an ID that is there several times.
func TestIRSpillSourceIsTheMemorySource(t *testing.T) {
	funcs := funcsWithDuplicates(rand.New(rand.NewSource(21)), 700)
	mem, disk := memOf(funcs), diskOf(t, funcs, SourceOptions{Cache: 1 << 20})
	defer disk.Release()
	if mem.Len() != disk.Len() {
		t.Fatalf("Len: %d, %d", mem.Len(), disk.Len())
	}
	for i := 0; i < mem.Len(); i++ {
		if a, b := mem.Head(i), disk.Head(i); a != b {
			t.Fatalf("Head(%d): %+v, %+v", i, a, b)
		}
	}
	// by number, in an order that is not the order of the program, and twice (the second may be from the cache)
	r := rand.New(rand.NewSource(22))
	for k := 0; k < 3000; k++ {
		i := r.Intn(mem.Len())
		ref := ir.Ref{ID: funcs[i].ID, Seq: i}
		g, err := disk.Func(ref)
		if err != nil {
			t.Fatal(err)
		}
		if d := Diff(funcs[i], g); d != "" {
			t.Fatalf("Func(%d) differs at %s", i, d)
		}
	}
	for _, f := range funcs {
		a, aok := mem.Lookup(f.ID)
		b, bok := disk.Lookup(f.ID)
		if a != b || aok != bok {
			t.Fatalf("Lookup(%s): %v %v, %v %v", f.ID, a, aok, b, bok)
		}
	}
	if _, ok := disk.Lookup("nothing"); ok {
		t.Error("Lookup of an ID that is not there")
	}
	var seqs []int
	if err := disk.Each(func(ref ir.Ref, f *ir.Func) error {
		if ref.ID != f.ID || Diff(funcs[ref.Seq], f) != "" {
			t.Fatalf("Each: %d is not the function", ref.Seq)
		}
		seqs = append(seqs, ref.Seq)
		return nil
	}); err != nil || len(seqs) != len(funcs) || !sort.IntsAreSorted(seqs) {
		t.Fatalf("Each: %d of %d, %v", len(seqs), len(funcs), err)
	}
	// what the readers of the heads of the IDs that are reached ask
	for i := range funcs {
		var want []ir.FuncID
		ir.Targets(funcs[i], func(id ir.FuncID) { want = append(want, id) })
		got := disk.Targets(i)
		// the file's list is the same IDs: the same set (an ID may be named twice in one)
		if !sameSet(want, got) {
			t.Fatalf("Targets(%d): %v, %v", i, want, got)
		}
	}
}

func sameSet(a, b []ir.FuncID) bool {
	m := map[ir.FuncID]bool{}
	for _, x := range a {
		m[x] = true
	}
	n := map[ir.FuncID]bool{}
	for _, x := range b {
		n[x] = true
	}
	if len(m) != len(n) {
		return false
	}
	for x := range m {
		if !n[x] {
			return false
		}
	}
	return true
}

// Two functions with one ID are two functions: neither replaces the other, and the number tells which is read.
func TestIRSpillSourceKeepsFunctionsWithOneID(t *testing.T) {
	r := rand.New(rand.NewSource(23))
	a, b, c := randomFunc(r, "p.F"), randomFunc(r, "p.F"), randomFunc(r, "p.G")
	disk := diskOf(t, []*ir.Func{a, c, b}, SourceOptions{})
	defer disk.Release()
	for seq, want := range map[int]*ir.Func{0: a, 2: b} {
		got, err := disk.Func(ir.Ref{ID: "p.F", Seq: seq})
		if err != nil || Diff(want, got) != "" {
			t.Errorf("number %d: %v", seq, err)
		}
	}
	if ref, _ := disk.Lookup("p.F"); ref.Seq != 2 {
		t.Errorf("Lookup is %v, want the last", ref)
	}
	if _, err := disk.Func(ir.Ref{ID: "p.F", Seq: 1}); err == nil {
		t.Error("the number of another function was read")
	}
}

func TestIRSpillSourceSortByID(t *testing.T) {
	r := rand.New(rand.NewSource(24))
	var funcs []*ir.Func
	for _, id := range []string{"p.C", "p.A", "p.B", "p.A", "p.D"} {
		funcs = append(funcs, randomFunc(r, id))
	}
	b := NewBuilder(NewIn(t.TempDir(), 0, nil), SourceOptions{})
	for _, f := range funcs {
		b.Add(f)
	}
	b.SortByID()
	d := b.Source(&ir.Program{})
	defer d.Release()
	var ids []string
	want := []*ir.Func{funcs[1], funcs[3], funcs[2], funcs[0], funcs[4]} // the two p.A in the order they came
	d.Each(func(ref ir.Ref, f *ir.Func) error {
		ids = append(ids, string(f.ID))
		if Diff(f, want[ref.Seq]) != "" {
			t.Errorf("number %d is another function", ref.Seq)
		}
		return nil
	})
	if strings.Join(ids, " ") != "p.A p.A p.B p.C p.D" {
		t.Errorf("order: %v", ids)
	}
	if h := d.Head(3); h.ID != "p.C" {
		t.Errorf("head 3: %+v", h)
	}
}

// A function that a reader writes to (Pin) is held in memory: the next read gives it with what was written, as the
// Source in memory does. A function that is not pinned is read again from the file, and the write is gone.
func TestIRSpillSourcePinKeepsWrites(t *testing.T) {
	r := rand.New(rand.NewSource(25))
	funcs := []*ir.Func{randomFunc(r, "p.A"), randomFunc(r, "p.B")}
	disk := diskOf(t, funcs, SourceOptions{Pin: func(f *ir.Func) bool { return f.ID == "p.A" }, Cache: -1})
	defer disk.Release()
	for _, c := range []struct {
		ref    ir.Ref
		stays  bool
		marker string
	}{{ir.Ref{ID: "p.A", Seq: 0}, true, "written"}, {ir.Ref{ID: "p.B", Seq: 1}, false, "written"}} {
		f, err := disk.Func(c.ref)
		if err != nil {
			t.Fatal(err)
		}
		f.Symbol = c.marker
		g, err := disk.Func(c.ref)
		if err != nil {
			t.Fatal(err)
		}
		if (g.Symbol == c.marker) != c.stays {
			t.Errorf("%s: the write stays: %v, want %v", c.ref.ID, g.Symbol == c.marker, c.stays)
		}
	}
	// Each gives the same function that Func gave
	disk.Each(func(ref ir.Ref, f *ir.Func) error {
		if ref.ID == "p.A" && f.Symbol != "written" {
			t.Error("Each gave a copy of the function that was written to")
		}
		return nil
	})
	// a function that is dropped is not there, and what is held for it goes
	disk.Drop(ir.Ref{ID: "p.A", Seq: 0})
	if _, err := disk.Func(ir.Ref{ID: "p.A", Seq: 0}); err == nil {
		t.Error("a dropped function was read")
	}
	if _, ok := disk.Lookup("p.A"); ok || !disk.Head(0).Gone {
		t.Error("a dropped function is still looked up")
	}
}

// The cache answers a function read lately with the same value, and holds no more than its limit.
func TestIRSpillSourceCache(t *testing.T) {
	r := rand.New(rand.NewSource(26))
	var funcs []*ir.Func
	for i := 0; i < 200; i++ {
		funcs = append(funcs, randomFunc(r, fmt.Sprint("p.F", i)))
	}
	disk := diskOf(t, funcs, SourceOptions{Cache: 200 << 10})
	defer disk.Release()
	ref := ir.Ref{ID: "p.F0", Seq: 0}
	a, _ := disk.Func(ref)
	b, _ := disk.Func(ref)
	if a != b {
		t.Error("a function read twice in a row is two copies")
	}
	for i, f := range funcs {
		if _, err := disk.Func(ir.Ref{ID: f.ID, Seq: i}); err != nil {
			t.Fatal(err)
		}
	}
	if disk.used > 200<<10+disk.lru.Back().Value.(*cacheEntry).w {
		t.Errorf("the cache holds %d bytes", disk.used)
	}
	if disk.Stats().CacheHits == 0 {
		t.Error("no read was answered by the cache")
	}
	// no cache: every read is a new copy
	none := diskOf(t, funcs[:3], SourceOptions{Cache: -1})
	defer none.Release()
	x, _ := none.Func(ir.Ref{ID: "p.F0", Seq: 0})
	y, _ := none.Func(ir.Ref{ID: "p.F0", Seq: 0})
	if x == y {
		t.Error("without a cache the same copy was given twice")
	}
}

// A function that cannot be read is an error that names the function: a record that was changed (the check of the
// record fails), a record of another function, a store that was closed, a number that is not there. The readers that
// cannot return an error stop with it (ir.Must, ir.Walk).
func TestIRSpillSourceReadFailures(t *testing.T) {
	r := rand.New(rand.NewSource(27))
	var funcs []*ir.Func
	for i := 0; i < 50; i++ {
		funcs = append(funcs, randomFunc(r, fmt.Sprint("p.F", i)))
	}
	disk := diskOf(t, funcs, SourceOptions{Cache: -1})
	disk.st.mu.Lock()
	disk.st.flush()
	l := disk.st.recs[7].loc
	disk.st.mu.Unlock()
	if _, err := disk.st.f.WriteAt([]byte{0xff, 0xff, 0xff}, l.off+headerLen); err != nil {
		t.Fatal(err)
	}
	bad := ir.Ref{ID: "p.F7", Seq: 7}
	if _, err := disk.Func(bad); !errors.Is(err, ErrCorrupt) {
		t.Errorf("a changed record: %v", err)
	}
	if _, err := disk.Func(ir.Ref{ID: "p.F8", Seq: 8}); err != nil {
		t.Errorf("the next one: %v", err)
	}
	if _, err := disk.Func(ir.Ref{ID: "p.F9", Seq: 8}); err == nil {
		t.Error("a number with the ID of another function")
	}
	if _, err := disk.Func(ir.Ref{ID: "p.F9", Seq: 99}); err == nil {
		t.Error("a number that is not there")
	}
	// Each stops at the first and says which
	err := disk.Each(func(ir.Ref, *ir.Func) error { return nil })
	var re *ir.ReadError
	if !errors.As(err, &re) || re.Ref != bad || !errors.Is(err, ErrCorrupt) {
		t.Errorf("Each: %v", err)
	}
	// the readers that cannot return an error panic with the error
	got := recoverReadError(func() { ir.Must(disk, bad) })
	if got == nil || got.Ref != bad {
		t.Errorf("Must: %v", got)
	}
	got = recoverReadError(func() { ir.Walk(disk, func(ir.Ref, *ir.Func) {}) })
	if got == nil || got.Ref != bad {
		t.Errorf("Walk: %v", got)
	}
	got = recoverReadError(func() { ir.WalkProject(disk, func(ir.Ref, *ir.Func) bool { return true }) })
	if got == nil && !allDep(disk) {
		t.Error("WalkProject: no panic")
	}
	// the file is read after it was closed
	disk.st.Close()
	if _, err := disk.Func(ir.Ref{ID: "p.F1", Seq: 1}); err == nil {
		t.Error("a read from a closed store")
	}
	// a released source is not read
	disk.Release()
	if _, err := disk.Func(ir.Ref{ID: "p.F1", Seq: 1}); err == nil {
		t.Error("a read after Release")
	}
}

func allDep(d *DiskSource) bool {
	for i := 0; i < d.Len(); i++ {
		if !d.Head(i).Dep {
			return false
		}
	}
	return true
}

func recoverReadError(fn func()) (re *ir.ReadError) {
	defer func() {
		if r := recover(); r != nil {
			re, _ = r.(*ir.ReadError)
			if re == nil {
				panic(r)
			}
		}
	}()
	fn()
	return nil
}

// The fault that a test puts in is the error of the read, from the n-th on.
func TestIRSpillSourceFault(t *testing.T) {
	r := rand.New(rand.NewSource(28))
	funcs := []*ir.Func{randomFunc(r, "p.A"), randomFunc(r, "p.B"), randomFunc(r, "p.C")}
	boom := errors.New("boom")
	disk := diskOf(t, funcs, SourceOptions{Cache: -1, Fault: func(n int) error {
		if n >= 2 {
			return boom
		}
		return nil
	}})
	defer disk.Release()
	if _, err := disk.Func(ir.Ref{ID: "p.A", Seq: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := disk.Func(ir.Ref{ID: "p.B", Seq: 1}); !errors.Is(err, boom) {
		t.Fatalf("the second read: %v", err)
	}
}

// PruneDepsOf drops what Program.PruneDeps drops, from a Source in memory and from one in the file (which does not
// read the functions for it).
func TestIRSpillPruneDepsOfIsPruneDeps(t *testing.T) {
	for seed := int64(0); seed < 30; seed++ {
		r := rand.New(rand.NewSource(seed))
		funcs := chain(r, 60)
		want := &ir.Program{Funcs: append([]*ir.Func(nil), funcs...)}
		want.PruneDeps()
		var wantIDs []string
		for _, f := range want.Funcs {
			wantIDs = append(wantIDs, string(f.ID))
		}
		mem := ir.MemSource(&ir.Program{Funcs: append([]*ir.Func(nil), funcs...)})
		disk := diskOf(t, funcs, SourceOptions{})
		for name, src := range map[string]ir.Source{"memory": mem, "file": disk} {
			ir.PruneDepsOf(src)
			var got []string
			src.Each(func(_ ir.Ref, f *ir.Func) error { got = append(got, string(f.ID)); return nil })
			if strings.Join(got, " ") != strings.Join(wantIDs, " ") {
				t.Fatalf("seed %d, %s:\n%v\n%v", seed, name, got, wantIDs)
			}
		}
		if disk.Stats().Reads > len(wantIDs) {
			t.Errorf("seed %d: the file was read %d times to prune, and %d remain", seed, disk.Stats().Reads-len(wantIDs), len(wantIDs))
		}
		disk.Release()
	}
}

// chain makes functions that call each other (a few IDs twice), some of a dependency.
func chain(r *rand.Rand, n int) []*ir.Func {
	var out []*ir.Func
	for i := 0; i < n; i++ {
		id := fmt.Sprint("p.F", i)
		if i > 5 && r.Intn(8) == 0 {
			id = fmt.Sprint("p.F", r.Intn(i))
		}
		f := &ir.Func{ID: ir.FuncID(id), Dep: r.Intn(3) != 0}
		var in []ir.Instr
		for k := r.Intn(4); k > 0; k-- {
			to := fmt.Sprint("p.F", r.Intn(n))
			switch r.Intn(3) {
			case 0:
				in = append(in, ir.Instr{Op: ir.OpCall, Callee: &ir.Callee{Kind: ir.CalleeStatic, Func: ir.FuncID(to)}})
			case 1:
				in = append(in, ir.Instr{Op: ir.OpCall, Callee: &ir.Callee{Kind: ir.CalleeDynamic, Candidates: []ir.FuncID{ir.FuncID(to), ir.FuncID(fmt.Sprint("p.F", r.Intn(n)))}}})
			default:
				in = append(in, ir.Instr{Op: ir.OpMakeClosure, Func: ir.FuncID(to)})
			}
		}
		f.Blocks = []ir.Block{{Instrs: in}}
		out = append(out, f)
	}
	return out
}

// Sources of two kinds, in memory and in the file, read as one.
func TestIRSpillConcatSource(t *testing.T) {
	r := rand.New(rand.NewSource(29))
	a, b := funcsWithDuplicates(r, 40), funcsWithDuplicates(r, 25)
	disk, mem := diskOf(t, a, SourceOptions{}), memOf(b)
	c := ir.ConcatSource(&ir.Program{}, disk, mem)
	all := append(append([]*ir.Func(nil), a...), b...)
	if c.Len() != len(all) {
		t.Fatalf("Len %d", c.Len())
	}
	for i, f := range all {
		g, err := c.Func(ir.Ref{ID: f.ID, Seq: i})
		if err != nil || Diff(f, g) != "" || c.Head(i).ID != f.ID {
			t.Fatalf("number %d: %v", i, err)
		}
	}
	// the last function with the ID, over the two
	for i, f := range all {
		ref, ok := c.Lookup(f.ID)
		if !ok || all[ref.Seq].ID != f.ID {
			t.Fatalf("Lookup %s", f.ID)
		}
		for j := i + 1; j < len(all); j++ {
			if all[j].ID == f.ID && ref.Seq < j {
				t.Fatalf("Lookup %s is %d, a later one is %d", f.ID, ref.Seq, j)
			}
		}
	}
	n := 0
	c.Each(func(ref ir.Ref, f *ir.Func) error {
		if ref.Seq != n || f.ID != all[n].ID {
			t.Fatalf("Each %d", n)
		}
		n++
		return nil
	})
	c.Drop(ir.Ref{ID: all[45].ID, Seq: 45})
	if !c.Head(45).Gone {
		t.Error("Drop")
	}
	c.Release()
}
