package valuecalls

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irssa"
)

// TestZeroCandidateCalls is a measure, not a check: how many calls of function values have no candidate, and where the called
// value comes from. It reads the IR of the benchmarks (the streams that the extractors write; `rivus debug extract` for Go), named
// in RIVUS_ZERO_CAND_IR (a comma separated list of files), and prints one table:
//
//	rivus debug extract ./... > go.jsonl            (Go)    dart run extractor/dart/bin/rivus_dart.dart <dir> > dart.jsonl
//	RIVUS_ZERO_CAND_IR=go.jsonl,dart.jsonl go test -run ZeroCandidateCalls -v ./internal/valuecalls
//
// A call is counted when the extractor gave no candidate, valuecalls found none, and no limit was hit (the call is left as
// "the called function is not known"). The columns say where the value comes from: a parameter (directly, or held in a
// field or an element of one), a captured variable, the result of a call (a function that returns a function, the next
// element of an iteration), a global, or something made there (an alloc, a constant). docs/ROADMAP.md has the numbers.
func TestZeroCandidateCalls(t *testing.T) {
	files := os.Getenv("RIVUS_ZERO_CAND_IR")
	if files == "" {
		t.Skip("RIVUS_ZERO_CAND_IR is not set (a measure; see the comment)")
	}
	for _, path := range strings.Split(files, ",") {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		prog, err := ir.Read(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		src := ir.MemSource(prog)
		Resolve(src)
		r := &resolver{src: src, prog: prog, forms: map[*ir.Func]*irssa.Form{}}
		counts := map[string]int{}
		total, zero := 0, 0
		for _, fn := range prog.Funcs {
			for bi := range fn.Blocks {
				for ii := range fn.Blocks[bi].Instrs {
					in := &fn.Blocks[bi].Instrs[ii]
					if in.Op != ir.OpCall || in.Callee == nil || in.Callee.Kind != ir.CalleeValue || len(in.Args) == 0 {
						continue
					}
					total++
					if len(in.Callee.ValueCands) > 0 || len(in.Callee.Candidates) > 0 || in.Callee.ValueLimit != "" {
						continue
					}
					form := r.form(fn)
					if form == nil {
						counts["no form"]++
						zero++
						continue
					}
					counts[r.originOfCalledValue(fn, form, form.Operand(bi, ii, 0))]++
					zero++
				}
			}
		}
		var keys []string
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := fmt.Sprintf("%s: %d calls of function values, %d with no candidate", path, total, zero)
		for _, k := range keys {
			out += fmt.Sprintf("; %s %d", k, counts[k])
		}
		t.Log(out)
	}
}

// originOfCalledValue says where the called value comes from (see the comment of TestZeroCandidateCalls).
func (r *resolver) originOfCalledValue(fn *ir.Func, form *irssa.Form, root irssa.Ref) string {
	seen := map[irssa.Ref]bool{}
	through := false
	res := ""
	var walk func(ref irssa.Ref)
	walk = func(ref irssa.Ref) {
		if seen[ref] || res != "" {
			return
		}
		seen[ref] = true
		if ref.Kind == irssa.RefInstr {
			in := &fn.Blocks[ref.Block].Instrs[ref.Index]
			switch in.Op {
			case ir.OpFieldLoad, ir.OpIndexLoad, ir.OpLoad:
				through = true
				if len(in.Args) > 0 {
					walk(form.Operand(ref.Block, ref.Index, 0))
					return
				}
			case ir.OpGlobalLoad:
				res = "global"
				return
			case ir.OpCall:
				res = "result of a call"
				return
			case ir.OpAlloc, ir.OpConst:
				res = "made there"
				return
			case ir.OpParam:
				if in.Index >= declared(fn) {
					res = "captured variable"
					return
				}
			}
		}
		k, _, from := r.classify(fn, form, ref)
		switch k {
		case refParam:
			if through {
				res = "parameter (held in a field or an element)"
			} else {
				res = "parameter"
			}
		case refFrom:
			for _, a := range from {
				walk(a)
			}
		case refClosure:
			res = "closure made there"
		default:
			res = "other"
		}
	}
	walk(root)
	if res == "" {
		res = "other"
	}
	return res
}
