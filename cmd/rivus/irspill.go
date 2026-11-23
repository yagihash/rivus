package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irspill"
)

// envIRSpill is the switch of the IR on disk (ADR-123): "1" keeps the functions of the Go code in a file, and the
// readers ask for them one at a time. It is not a flag and is not in -h: the result is the same, and the switch goes
// away when the disk is the default.
const envIRSpill = "RIVUS_INTERNAL_IR_SPILL"

// envIRSpillFault is for the tests of the failure to read: with a number n, every read of a function from the file from
// the n-th on fails. It is not a flag and is not in -h.
const envIRSpillFault = "RIVUS_INTERNAL_IR_SPILL_FAULT"

func spillOptions() irspill.SourceOptions {
	var o irspill.SourceOptions
	if n, err := strconv.Atoi(os.Getenv(envIRSpillFault)); err == nil && n > 0 {
		o.Fault = func(read int) error {
			if read >= n {
				return fmt.Errorf("a failure put in by %s=%d", envIRSpillFault, n)
			}
			return nil
		}
	}
	return o
}

func newSpillBuilder() *irspill.Builder {
	return irspill.NewBuilder(irspill.New(0, os.Stderr), spillOptions())
}

// spillProgram puts the functions of a program, made in memory, into a file: the program is then the header of the
// Source.
func spillProgram(p *ir.Program) *irspill.DiskSource {
	return irspill.FromProgram(irspill.New(0, os.Stderr), p, spillOptions())
}

// readSpill reads a stream of the IR (an extractor's, or a file's) a line at a time into a file: the functions are put
// as they are decoded and are not kept. It returns the program without its functions, and the Source that has them (the
// dependencies that nothing of the project reaches are already dropped, as PruneDeps does for a program in memory).
// When the decoding fails, what is left of r is read and dropped, so that a writer is never left to block.
func readSpill(r io.Reader) (*ir.Program, *irspill.DiskSource, error) {
	b := newSpillBuilder()
	prog, err := ir.ReadFuncs(r, func(f *ir.Func) error {
		b.Add(f)
		return nil
	})
	if err != nil {
		io.Copy(io.Discard, r)
		b.Discard()
		return nil, nil, err
	}
	disk := b.Source(prog)
	ir.PruneDepsOf(disk)
	return prog, disk, nil
}

// readIRSpill is readIR with the functions in a file.
func readIRSpill(path string) (*loaded, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	prog, disk, err := readSpill(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &loaded{ir: prog, texts: ir.NewStreamTexts(prog), src: disk, disk: disk}, nil
}

// noteSpill says on w what the file was used for (-verbose).
func noteSpill(w io.Writer, d *irspill.DiskSource) {
	if d == nil {
		return
	}
	s := d.Stats()
	fmt.Fprintf(w, "rivus: the IR on disk: %d functions, %d MB written (%d held in memory), %d calls with the candidates of valuecalls in the table; put %v, %d reads (%d answered by the cache) %v\n",
		s.Store.Funcs, s.Store.Bytes>>20, s.Store.MemFuncs, s.Updates, s.PutTime.Round(time.Millisecond), s.Reads, s.CacheHits, s.ReadTime.Round(time.Millisecond))
}
