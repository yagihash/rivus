// Package to_later is a cell of the zoo.
//
// @cell(F/clos/to-later)
// @shape: a closure that captures a value is given to a function of the standard library that calls it later, or in another goroutine (sync.WaitGroup.Go, time.AfterFunc, context.AfterFunc)
package to_later

import (
	"context"
	"sync"
	"time"

	"zoo/mark"
)

func init() {
	mark.Case("F/clos/to-later/vuln", vuln)
	mark.Case("F/clos/to-later/vuln2", vuln2)
	mark.Case("F/clos/to-later/vuln3", vuln3)
	mark.Case("F/clos/to-later/fixed", fixed)
	mark.Case("F/clos/to-later/decoy", decoy)
}

func vuln() { // @vuln
	v := mark.Src("clos-to-later-vuln") // @source(clos-to-later-vuln)
	var wg sync.WaitGroup
	wg.Go(func() {
		mark.SinkCmd(v) // @sink(clos-to-later-vuln)
	})
	wg.Wait()
}

// vuln2: the timer calls the function after the delay.
func vuln2() { // @vuln
	v := mark.Src("clos-to-later-vuln2") // @source(clos-to-later-vuln2)
	done := make(chan struct{})
	time.AfterFunc(time.Millisecond, func() {
		mark.SinkCmd(v) // @sink(clos-to-later-vuln2)
		close(done)
	})
	<-done
}

// vuln3: the context calls the function in its own goroutine when it is cancelled.
func vuln3() { // @vuln
	v := mark.Src("clos-to-later-vuln3") // @source(clos-to-later-vuln3)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	context.AfterFunc(ctx, func() {
		mark.SinkCmd(v) // @sink(clos-to-later-vuln3)
		close(done)
	})
	cancel()
	<-done
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("clos-to-later-fixed"))
	var wg sync.WaitGroup
	wg.Go(func() {
		mark.SinkCmd(v) // @sink(none)
	})
	wg.Wait()
	done := make(chan struct{})
	time.AfterFunc(time.Millisecond, func() {
		mark.SinkCmd(v) // @sink(none)
		close(done)
	})
	<-done
}

func decoy() { // @decoy
	a := mark.Src("clos-to-later-decoy") // @source(clos-to-later-decoy)
	k := "constant"
	var wg sync.WaitGroup
	wg.Go(func() {
		mark.SinkCmd(k) // @sink(none) @nosink(clos-to-later-decoy)
	})
	wg.Wait()
	done := make(chan struct{})
	time.AfterFunc(time.Millisecond, func() {
		mark.SinkCmd(k) // @sink(none) @nosink(clos-to-later-decoy)
		close(done)
	})
	<-done
	ctx, cancel := context.WithCancel(context.Background())
	done2 := make(chan struct{})
	context.AfterFunc(ctx, func() {
		mark.SinkCmd(k) // @sink(none) @nosink(clos-to-later-decoy)
		close(done2)
	})
	cancel()
	<-done2
	mark.SinkSQL(a) // @sink(clos-to-later-decoy)
}
