package irspill

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/ir"
)

// fill sets every field that the stream has (not the ones with `json:"-"`), at random, so that a field that a
// later change adds and the record does not keep is found by the round trip.
func fill(r *rand.Rand, v reflect.Value, depth int) {
	switch v.Kind() {
	case reflect.Pointer:
		if r.Intn(4) == 0 {
			return
		}
		v.Set(reflect.New(v.Type().Elem()))
		fill(r, v.Elem(), depth+1)
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Tag.Get("json") == "-" {
				continue
			}
			fill(r, v.Field(i), depth+1)
		}
	case reflect.Slice:
		n := r.Intn(4)
		if depth > 6 {
			n = 0
		}
		if n == 0 {
			return
		}
		v.Set(reflect.MakeSlice(v.Type(), n, n))
		for i := 0; i < n; i++ {
			fill(r, v.Index(i), depth+1)
		}
	case reflect.String:
		words := []string{"", "a", "x/y.Z", "(*pkg.T).M", "string", "[]*pkg.T", "name", "f.go", "g.go", "héllo\x00\xff", strings.Repeat("c", 300)}
		if r.Intn(5) == 0 {
			v.SetString(fmt.Sprint("s", r.Intn(1000)))
		} else {
			v.SetString(words[r.Intn(len(words))])
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(r.Intn(2000000) - 1000000))
		if r.Intn(2) == 0 {
			v.SetInt(int64(r.Intn(10) - 1))
		}
	case reflect.Bool:
		v.SetBool(r.Intn(2) == 0)
	}
}

func randomFunc(r *rand.Rand, id string) *ir.Func {
	f := &ir.Func{}
	fill(r, reflect.ValueOf(f).Elem(), 0)
	f.ID = ir.FuncID(id)
	// the extra blocks make the long records of a real function
	for i := r.Intn(30); i > 0; i-- {
		var b ir.Block
		fill(r, reflect.ValueOf(&b).Elem(), 3)
		f.Blocks = append(f.Blocks, b)
	}
	return f
}

func TestIRSpillRoundTripRandom(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 400; i++ {
		f := randomFunc(r, fmt.Sprint("f", i))
		rec := Encode(nil, f)
		got, n, err := Decode(rec)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		if n != len(rec) {
			t.Fatalf("%d: took %d of %d bytes", i, n, len(rec))
		}
		if d := Diff(f, got); d != "" {
			t.Fatalf("%d: differs at %s", i, d)
		}
	}
}

func TestIRSpillRoundTripShapes(t *testing.T) {
	// what the zero values and the unknown names do
	cases := map[string]*ir.Func{
		"empty": {ID: "p.F"},
		"unknown names": {ID: "p.G", Params: []ir.Param{{Class: "weird"}}, Blocks: []ir.Block{{
			ID: 7, Term: ir.Term{Kind: "newkind", Cond: ir.NoLocal},
			Succs:  []ir.Edge{{To: -1, Kind: "other"}},
			Instrs: []ir.Instr{{Op: "newop", Const: &ir.Const{Kind: "newconst"}, Callee: &ir.Callee{Kind: "k"}, Path: []ir.Step{{Kind: "z"}}}},
		}}},
		"empty callee and const":     {ID: "p.H", Blocks: []ir.Block{{Instrs: []ir.Instr{{Op: ir.OpCall, Callee: &ir.Callee{}, Const: &ir.Const{}}}}}},
		"zero positions in the list": {ID: "p.I", Blocks: []ir.Block{{Instrs: []ir.Instr{{Op: ir.OpCall, ArgPos: []ir.Pos{{}, {File: "a", Offset: 5}, {}}}}}}},
	}
	for name, f := range cases {
		got, _, err := Decode(Encode(nil, f))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if d := Diff(f, got); d != "" {
			t.Errorf("%s: differs at %s", name, d)
		}
	}
}

func TestIRSpillRecordIsSmall(t *testing.T) {
	// a load, a store and a call with a position: a few bytes each
	f := &ir.Func{ID: "p.F", File: "p.go"}
	var in []ir.Instr
	for i := 0; i < 100; i++ {
		in = append(in, ir.Instr{Op: ir.OpLoad, Dsts: []ir.LocalID{ir.LocalID(i + 1)}, Args: []ir.LocalID{ir.LocalID(i)}, Pos: ir.Pos{File: "p.go", Offset: 100 + i*7, Length: 5, Line: 3 + i/4, Col: 4}})
	}
	f.Blocks = []ir.Block{{Instrs: in, Term: ir.Term{Kind: ir.TermReturn}}}
	if n := len(Encode(nil, f)); n > 100*14 {
		t.Errorf("100 loads take %d bytes", n)
	}
}

func TestIRSpillRefusesCorrupt(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	f := randomFunc(r, "p.F")
	rec := Encode(nil, f)

	// every byte changed: the CRC (or the length) says no
	for i := range rec {
		bad := append([]byte(nil), rec...)
		bad[i] ^= 0x55
		if _, _, err := Decode(bad); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("byte %d changed: err = %v", i, err)
		}
	}
	// cut short
	for _, n := range []int{0, 3, headerLen - 1, headerLen, len(rec) - 1} {
		if _, _, err := Decode(rec[:n]); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%d bytes: err = %v", n, err)
		}
	}
	// a length over the limit is refused before anything is made for it
	big := append([]byte(nil), rec...)
	binary.LittleEndian.PutUint32(big, MaxRecord+1)
	if _, _, err := Decode(big); !errors.Is(err, ErrCorrupt) || !strings.Contains(err.Error(), "length") {
		t.Errorf("a length over the limit: err = %v", err)
	}
	// a length that is more than the bytes there are
	binary.LittleEndian.PutUint32(big, uint32(len(rec)))
	if _, _, err := Decode(big); !errors.Is(err, ErrCorrupt) {
		t.Errorf("a length over the bytes: err = %v", err)
	}
}

// reseal gives the body a right CRC again: a body that is wrong and sealed, as a bug of the writer would make.
func reseal(rec []byte) {
	binary.LittleEndian.PutUint32(rec[4:], crc32.Checksum(rec[headerLen:], crcTable))
}

func TestIRSpillSealedGarbageIsAnErrorNotAPanic(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	f := randomFunc(r, "p.F")
	rec := Encode(nil, f)
	for i := 0; i < 5000; i++ {
		bad := append([]byte(nil), rec...)
		for k := r.Intn(4) + 1; k > 0; k-- {
			bad[headerLen+r.Intn(len(bad)-headerLen)] = byte(r.Intn(256))
		}
		reseal(bad)
		Decode(bad) // an error or a function; no panic
	}
}

func FuzzIRSpillDecode(f *testing.F) {
	r := rand.New(rand.NewSource(4))
	f.Add(Encode(nil, randomFunc(r, "p.F")))
	f.Add(Encode(nil, &ir.Func{ID: "p.G"}))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > headerLen {
			// give the body the length and the CRC it has, so that the reader goes past the check
			c := append([]byte(nil), b...)
			binary.LittleEndian.PutUint32(c, uint32(len(c)-headerLen))
			reseal(c)
			Decode(c)
		}
		Decode(b)
	})
}

func put(t *testing.T, s *Store, r *rand.Rand, n int, prefix ...string) []*ir.Func {
	p := "p.F"
	if len(prefix) > 0 {
		p = prefix[0]
	}
	t.Helper()
	var out []*ir.Func
	for i := 0; i < n; i++ {
		f := randomFunc(r, fmt.Sprint(p, i))
		s.Put(f)
		out = append(out, f)
	}
	return out
}

func same(t *testing.T, s *Store, want []*ir.Func) {
	t.Helper()
	for _, f := range want {
		got, err := s.Get(f.ID)
		if err != nil {
			t.Fatalf("%s: %v", f.ID, err)
		}
		if d := Diff(f, got); d != "" {
			t.Fatalf("%s: differs at %s", f.ID, d)
		}
	}
	i := 0
	if err := s.Each(func(g *ir.Func) error {
		if g.ID != want[i].ID {
			t.Fatalf("Each: %d is %s, want %s", i, g.ID, want[i].ID)
		}
		i++
		return nil
	}); err != nil || i != len(want) {
		t.Fatalf("Each: %d functions, %v", i, err)
	}
}

func TestIRSpillStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	var warn bytes.Buffer
	s := NewIn(dir, 0, &warn)
	defer s.Close()
	want := put(t, s, rand.New(rand.NewSource(5)), 3000) // more than the buffer: some are written, some wait
	same(t, s, want)
	st := s.Stats()
	if st.Funcs != 3000 || st.MemFuncs != 0 || st.Fallback != "" || st.Bytes < flushAt {
		t.Errorf("stats = %+v", st)
	}
	if warn.Len() != 0 {
		t.Errorf("warned: %s", warn.String())
	}
	if _, err := s.Get("nothing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get of an ID that was not put: %v", err)
	}
	// the same ID again replaces
	g := randomFunc(rand.New(rand.NewSource(6)), "p.F0")
	s.Put(g)
	got, err := s.Get("p.F0")
	if err != nil || Diff(g, got) != "" {
		t.Errorf("replace: %v", err)
	}
	if n := len(s.IDs()); n != 3000 {
		t.Errorf("%d IDs after a replace", n)
	}
}

func TestIRSpillStoreConcurrentGet(t *testing.T) {
	s := NewIn(t.TempDir(), 0, nil)
	defer s.Close()
	want := put(t, s, rand.New(rand.NewSource(7)), 600)
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := w; i < len(want); i += 3 {
				got, err := s.Get(want[i].ID)
				if err != nil || Diff(want[i], got) != "" {
					t.Errorf("%s: %v", want[i].ID, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
}

func TestIRSpillStoreLimitHoldsInMemory(t *testing.T) {
	dir := t.TempDir()
	var warn bytes.Buffer
	s := NewIn(dir, 400000, &warn) // a few functions
	defer s.Close()
	want := put(t, s, rand.New(rand.NewSource(8)), 200)
	same(t, s, want)
	st := s.Stats()
	if st.MemFuncs == 0 || st.MemFuncs == 200 || st.Bytes > 400000 || !strings.Contains(st.Fallback, "limit") {
		t.Errorf("stats = %+v", st)
	}
	if strings.Count(warn.String(), "\n") != 1 || !strings.Contains(warn.String(), "held in memory") {
		t.Errorf("the warning must be one line: %q", warn.String())
	}
}

func TestIRSpillStoreWriteFailureHoldsInMemory(t *testing.T) {
	var warn bytes.Buffer
	s := NewIn(t.TempDir(), 0, &warn)
	defer s.Close()
	r := rand.New(rand.NewSource(9))
	want := put(t, s, r, 1500)
	// the file goes (the disk, as far as the store can tell): the next write fails
	s.f.Close()
	want = append(want, put(t, s, r, 1500, "r.F")...)
	for i := len(want); i < len(want)+3000 && s.Stats().Fallback == ""; i++ { // enough to reach a write
		f := randomFunc(r, fmt.Sprint("q.F", i))
		s.Put(f)
		want = append(want, f)
	}
	st := s.Stats()
	if st.Fallback == "" || st.MemFuncs == 0 || !strings.Contains(warn.String(), "held in memory") {
		t.Fatalf("stats = %+v, warning %q", st, warn.String())
	}
	// what was in the file before it went cannot be read: Get says so, and does not give a wrong function
	if _, err := s.Get(want[0].ID); err == nil {
		t.Errorf("a function of the file that is gone was read")
	}
	// every function that was not written is there, with the same content
	for _, f := range want {
		if got, err := s.Get(f.ID); err == nil && Diff(f, got) != "" {
			t.Fatalf("%s: differs at %s", f.ID, Diff(f, got))
		}
	}
	var lost int
	for _, f := range want {
		if _, err := s.Get(f.ID); err != nil {
			lost++
		}
	}
	if lost == 0 || lost == len(want) {
		t.Errorf("%d of %d functions are not readable", lost, len(want))
	}
}

func TestIRSpillStoreNoFileHoldsInMemory(t *testing.T) {
	var warn bytes.Buffer
	s := NewIn(filepath.Join(t.TempDir(), "no", "such", "dir"), 0, &warn)
	defer s.Close()
	want := put(t, s, rand.New(rand.NewSource(10)), 50)
	same(t, s, want)
	if !strings.Contains(warn.String(), "held in memory") || s.Stats().MemFuncs != 50 {
		t.Errorf("stats = %+v, warning %q", s.Stats(), warn.String())
	}
}

func TestIRSpillReadFailureIsAnError(t *testing.T) {
	s := NewIn(t.TempDir(), 0, nil)
	defer s.Close()
	want := put(t, s, rand.New(rand.NewSource(11)), 2000)
	// one record is changed in the file
	s.mu.Lock()
	l := s.index[want[0].ID]
	s.mu.Unlock()
	if _, err := s.f.WriteAt([]byte{0xff, 0xff}, l.off+headerLen); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(want[0].ID); !errors.Is(err, ErrCorrupt) {
		t.Errorf("a changed record: %v", err)
	}
	if err := s.Each(func(*ir.Func) error { return nil }); !errors.Is(err, ErrCorrupt) {
		t.Errorf("Each over a changed record: %v", err)
	}
	if _, err := s.Get(want[1].ID); err != nil {
		t.Errorf("the next record: %v", err)
	}
}

func TestIRSpillTempFileHasNoNameAndMode0600(t *testing.T) {
	dir := t.TempDir()
	s := NewIn(dir, 0, nil)
	defer s.Close()
	put(t, s, rand.New(rand.NewSource(12)), 2000) // some are written
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != 0 {
		t.Fatalf("the directory has %v (%v): the file must have no name", ents, err)
	}
	st, err := s.f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", st.Mode().Perm())
	}
	if st.Size() == 0 {
		t.Errorf("nothing was written")
	}
}

func TestIRSpillUsesTMPDIR(t *testing.T) {
	// a TMPDIR that is not there: the store says so and holds in memory. A TMPDIR that is there: it works, in it.
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "gone"))
	var warn bytes.Buffer
	s := New(0, &warn)
	s.Close()
	if !strings.Contains(warn.String(), "gone") {
		t.Errorf("the temporary directory was not TMPDIR: %q", warn.String())
	}
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	warn.Reset()
	s = New(0, &warn)
	defer s.Close()
	put(t, s, rand.New(rand.NewSource(13)), 5)
	if warn.Len() != 0 || s.f == nil {
		t.Errorf("with a TMPDIR that is there: %q", warn.String())
	}
}

// TestIRSpillKilledProcessLeavesNothing runs the helper below in a process, kills it with SIGKILL while it holds a
// store with data in it, and looks in the temporary directory.
func TestIRSpillKilledProcessLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestIRSpillHelperProcess$")
	cmd.Env = append(os.Environ(), "IRSPILL_HELPER=1", "TMPDIR="+dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "ready ") {
				line <- sc.Text()
				return
			}
		}
		line <- ""
	}()
	select {
	case l := <-line:
		if !strings.HasPrefix(l, "ready ") || l == "ready 0" {
			t.Fatalf("the helper said %q", l)
		}
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		t.Fatal("the helper did not say ready")
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Errorf("while the process lives, the directory has %v", ents)
	}
	if err := cmd.Process.Kill(); err != nil { // SIGKILL
		t.Fatal(err)
	}
	cmd.Wait()
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Errorf("after the kill, the directory has %v", ents)
	}
}

func TestIRSpillHelperProcess(t *testing.T) {
	if os.Getenv("IRSPILL_HELPER") != "1" {
		t.Skip("run by TestIRSpillKilledProcessLeavesNothing")
	}
	s := New(0, os.Stderr)
	put(t, s, rand.New(rand.NewSource(14)), 2000)
	fmt.Printf("ready %d\n", s.Stats().Bytes)
	time.Sleep(time.Minute)
}
