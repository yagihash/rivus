package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irspill"
	"github.com/yagihash/rivus/internal/valuecalls"
)

// envIRSpill is the switch of the IR on disk (ADR-123): "1" keeps the functions of the Go code in a file, and the
// readers ask for them one at a time. It is not a flag and is not in -h: the result is the same, and the switch goes
// away when the disk is the default.
const envIRSpill = "RIVUS_INTERNAL_IR_SPILL"

// envIRSpillFault is for the tests of the failure to read: with a number n, every read of a function from the file from
// the n-th on fails. It is not a flag and is not in -h.
const envIRSpillFault = "RIVUS_INTERNAL_IR_SPILL_FAULT"

func spillOptions() irspill.SourceOptions {
	o := irspill.SourceOptions{Pin: valuecalls.Writes}
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

// noteSpill says on w what the file was used for (-verbose).
func noteSpill(w io.Writer, d *irspill.DiskSource) {
	if d == nil {
		return
	}
	s := d.Stats()
	fmt.Fprintf(w, "rivus: the IR on disk: %d functions, %d MB written (%d held in memory), %d held for the writes of valuecalls; put %v, %d reads (%d answered by the cache) %v\n",
		s.Store.Funcs, s.Store.Bytes>>20, s.Store.MemFuncs, s.Pinned, s.PutTime.Round(time.Millisecond), s.Reads, s.CacheHits, s.ReadTime.Round(time.Millisecond))
}
