package taint

import (
	"reflect"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/testutil"
)

// bigOffset is past what 32 bits hold. The Rust extractor makes the offset of a
// position as line*100000+col, so it is this large from about the 21,475th
// line of a file.
const bigOffset = 2_400_400_013

// shiftOffsets adds d to every offset in v: the positions of the program and
// the offsets that its texts are told by.
func shiftOffsets(v reflect.Value, d int, seen map[uintptr]bool) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		shiftOffsets(v.Elem(), d, seen)
	case reflect.Interface:
		if !v.IsNil() {
			shiftOffsets(v.Elem(), d, seen)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			shiftOffsets(v.Index(i), d, seen)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			if e := v.MapIndex(k); e.Kind() == reflect.Pointer || e.Kind() == reflect.Slice {
				shiftOffsets(e, d, seen)
			}
		}
	case reflect.Struct:
		switch x := v.Addr().Interface().(type) {
		case *ir.Pos:
			if *x != (ir.Pos{}) {
				x.Offset += d
			}
			return
		case *ir.TextGroup:
			for i := range x.Offsets {
				x.Offsets[i] += d
			}
			return
		case *ir.CallText:
			x.Offset += d
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				shiftOffsets(v.Field(i), d, seen)
			}
		}
	}
}

// runShifted runs a case with the texts that went through the stream, with every
// offset moved by d.
func runShifted(t *testing.T, name string, d int) *Result {
	t.Helper()
	l := loadIR(t, testutil.Testdata(), "./cases/"+name+"/...")
	rules := testutil.Rules(t)
	rules.Expand(l.gossa, 0)
	goir.AddTexts(l.texts.(*goir.Texts), l.ir)
	if d != 0 {
		shiftOffsets(reflect.ValueOf(l.ir), d, map[uintptr]bool{})
	}
	g, err := sg.Build(ir.MemSource(l.ir))
	if err != nil {
		t.Fatal(err)
	}
	l.sg, l.texts = g, ir.NewStreamTexts(l.ir)
	return l.engine(l.sites(rules), rules, DefaultOptions()).Run()
}

// A position whose offset does not fit in 32 bits keeps its offset and its text
// in the steps of a path and in the notes (the places that hold a position in
// the packed form), as the same position with a small offset does.
func TestTheStepsOfAPathKeepALargeOffsetAndItsText(t *testing.T) {
	for _, name := range []string{"inter", "heap"} {
		t.Run(name, func(t *testing.T) {
			small, big := runShifted(t, name, 0), runShifted(t, name, bigOffset)
			if len(small.Findings) == 0 || len(small.Findings) != len(big.Findings) {
				t.Fatalf("findings: %d with small offsets, %d with large", len(small.Findings), len(big.Findings))
			}
			steps, notes := 0, 0
			for i, f := range small.Findings {
				g := big.Findings[i]
				if len(f.Paths) != len(g.Paths) {
					t.Fatalf("finding %d: %d paths, then %d", i, len(f.Paths), len(g.Paths))
				}
				for j, p := range f.Paths {
					if len(p.Steps) != len(g.Paths[j].Steps) {
						t.Fatalf("finding %d path %d: %d steps, then %d", i, j, len(p.Steps), len(g.Paths[j].Steps))
					}
					for k, st := range p.Steps {
						bs := g.Paths[j].Steps[k]
						if st.Position.Offset == 0 {
							continue
						}
						steps++
						if bs.Position.Offset != st.Position.Offset+bigOffset {
							t.Errorf("step offset %d, want %d", bs.Position.Offset, st.Position.Offset+bigOffset)
						}
						if bs.Snippet != st.Snippet {
							t.Errorf("step snippet %q, want %q (offset %d)", bs.Snippet, st.Snippet, bs.Position.Offset)
						}
					}
				}
				if len(f.Uncertainties) != len(g.Uncertainties) {
					t.Fatalf("finding %d: %d notes, then %d", i, len(f.Uncertainties), len(g.Uncertainties))
				}
				for k, u := range f.Uncertainties {
					bu := g.Uncertainties[k]
					if u.Position.Offset == 0 {
						continue
					}
					notes++
					if bu.Position.Offset != u.Position.Offset+bigOffset || bu.Snippet != u.Snippet {
						t.Errorf("note %q at %d, want %q at %d", bu.Snippet, bu.Position.Offset, u.Snippet, u.Position.Offset+bigOffset)
					}
				}
			}
			if steps == 0 {
				t.Fatal("no step with an offset: the test checks nothing")
			}
			t.Logf("%d steps and %d notes with large offsets checked", steps, notes)
		})
	}
}

// The places that read the text of a position that does not go through the packed
// form (the unresolved values of a provenance) keep a large offset and its text
// too.
func TestAProvenanceKeepsALargeOffsetAndItsText(t *testing.T) {
	small, big := runShifted(t, "guard", 0), runShifted(t, "guard", bigOffset)
	if len(small.Findings) != len(big.Findings) {
		t.Fatalf("findings: %d, then %d", len(small.Findings), len(big.Findings))
	}
	checked := 0
	for i, f := range small.Findings {
		for j, n := range f.ConditionalNodes {
			if n.Provenance == nil {
				continue
			}
			bn := big.Findings[i].ConditionalNodes[j].Provenance
			if bn == nil || len(bn.Unresolved) != len(n.Provenance.Unresolved) {
				t.Fatalf("finding %d node %d: the provenance differs", i, j)
			}
			for k, u := range n.Provenance.Unresolved {
				bu := bn.Unresolved[k]
				if u.Position.Offset == 0 {
					continue
				}
				checked++
				if bu.Position.Offset != u.Position.Offset+bigOffset || bu.Snippet != u.Snippet || u.Snippet == "" {
					t.Errorf("unresolved %q at %d, want %q at %d", bu.Snippet, bu.Position.Offset, u.Snippet, u.Position.Offset+bigOffset)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no unresolved value with a position: the test checks nothing")
	}
}
