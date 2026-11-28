package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// notifyContext is a context that is cancelled when rivus gets SIGINT, SIGTERM or SIGHUP, for a run that starts a program of its own (the Rust
// build under the sandbox). That program is in a session of its own, so the Ctrl-C of the terminal does not reach it, and its time limit is kept
// by rivus: when rivus ended at once, the build went on (a build script that loops and writes, up to the file size limit), and the work
// directory was not removed. With the context cancelled, supervise kills the group that it started (and only that group) and the
// deferred cleanups run. stop gives the signals back to their default and ends the context.
func notifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	ctx, stop := cancelOn(parent, ch)
	return ctx, func() {
		signal.Stop(ch)
		stop()
	}
}

// cancelOn is the context of parent that is cancelled when a signal arrives on sig, or when parent ends, or when stop is called.
func cancelOn(parent context.Context, sig <-chan os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-sig:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}
