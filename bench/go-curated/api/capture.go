package api

import (
	"curated/sys"
	"curated/valid"
	"curated/web"
)

// The values a closure captures, and where the closure is called. A closure that is called where it is
// made (`go func(){...}()`, `defer`, `func(){...}()`) or that gets its values by its own arguments
// is found. A closure given to a helper that calls it (`run(func() { ... a ... })`) is not: the
// helper calls a function value, and the values it captured are known only where it was made
// (C-7, docs/DESIGN-capture.md). The `@miss(silent)` lines are that case; the other lines are the
// forms that are found, kept here to show the difference, and the forms a change must keep quiet.

// runCb is a helper that calls what it is given (db.WithTx and every Do/Run/Each of the real projects).
func runCb(f func()) { f() }

// runWith calls f with a constant: what the closure is given is not what it captured.
func runWith(f func(string)) { f("fixed") }

// runArg gives f the value it was called with.
func runArg(s string, f func(string)) { f(s) }

// runTwo calls both.
func runTwo(f, g func()) {
	f()
	g()
}

// Captures runs the shapes of this file.
func (h *Handlers) Captures(c *web.Context) {
	capRunA(c)
	capRunB(c)
	capConst(c)
	capParam(c)
	capArg(c)
	capInline(c)
	capGo(c)
	capDefer(c)
	capWrite(c)
	capWriteConst(c)
	capNested(c)
	capLoop(c)
	capTwo(c)
	capNever(c)
	capReturned(c)
	capLate(c)
	capHandler(c, &web.RouterGroup{})
	capSeenAndPassed(c)
	capFuncCall(c)
	capSanitized(c)
}

// capRunA and capRunB: one helper, two callers, each with its own captured value. Neither value goes
// into the closure of the other.
func capRunA(c *web.Context) {
	a := c.Query("a") // @source(cap-run-a) @miss(silent)
	runCb(func() {
		sys.Run(a) // @sink(cap-run-a) @nosink(cap-run-b)
	})
}

func capRunB(c *web.Context) {
	b := c.Query("b") // @source(cap-run-b) @miss(silent)
	runCb(func() {
		sys.Run(b) // @sink(cap-run-b) @nosink(cap-run-a)
	})
}

// capConst: the closure runs a constant, and the value that was captured goes to the sink by the
// other way (the closure does not make it reach the sink in the closure).
func capConst(c *web.Context) {
	a := c.Query("const") // @source(cap-const)
	runCb(func() {
		sys.Run("fixed") // @sink(none)
	})
	sys.Run(a) // @sink(cap-const)
}

// capParam: the closure uses its own parameter, which the helper gives (a constant); the value the
// function holds is not captured. A rule that gave the parameter of a closure the taint of the
// captured values would report this line.
func capParam(c *web.Context) {
	a := c.Query("param") // @source(cap-param)
	runWith(func(s string) {
		sys.Run(s) // @sink(none)
	})
	sys.Run(a) // @sink(cap-param)
}

// capArg: the helper gives the closure the value (the closure's own parameter): found today.
func capArg(c *web.Context) {
	a := c.Query("arg") // @source(cap-arg)
	runArg(a, func(s string) {
		sys.Run(s) // @sink(cap-arg)
	})
}

// capInline: made and called in one place (found).
func capInline(c *web.Context) {
	a := c.Query("inline") // @source(cap-inline)
	func() {
		sys.Run(a) // @sink(cap-inline)
	}()
}

// capGo and capDefer: the closure is called where it was made (found).
func capGo(c *web.Context) {
	a := c.Query("go") // @source(cap-go)
	go func() {
		sys.Run(a) // @sink(cap-go)
	}()
}

func capDefer(c *web.Context) {
	a := c.Query("defer") // @source(cap-defer)
	defer func() {
		sys.Run(a) // @sink(cap-defer)
	}()
}

// capWrite: the closure writes a variable it captured, and the function uses it after the helper.
func capWrite(c *web.Context) {
	var out string
	a := c.Query("write") // @source(cap-write) @miss(silent)
	runCb(func() {
		out = a
	})
	sys.Run(out) // @sink(cap-write)
}

// capWriteConst: the closure writes a constant: the variable is not the value that was captured.
func capWriteConst(c *web.Context) {
	var out string
	a := c.Query("writeconst") // @source(cap-write-const)
	runCb(func() {
		out = "fixed"
	})
	sys.Run(out) // @sink(none)
	sys.Run(a)   // @sink(cap-write-const)
}

// capNested: the closure in the closure captures what the outer closure captured.
func capNested(c *web.Context) {
	a := c.Query("nested") // @source(cap-nested) @miss(silent)
	runCb(func() {
		runCb(func() {
			sys.Run(a) // @sink(cap-nested)
		})
	})
}

// capLoop: a closure made for each element of a loop; each captures its element.
func capLoop(c *web.Context) {
	names := []string{c.Query("loop")} // @source(cap-loop) @miss(silent)
	for _, n := range names {
		runCb(func() {
			sys.Run(n) // @sink(cap-loop)
		})
	}
}

// capTwo: two closures of one call, each with its own value (the helper calls both).
func capTwo(c *web.Context) {
	a := c.Query("two-a") // @source(cap-two-a) @miss(silent)
	b := "fixed"
	runTwo(func() {
		sys.Run(a) // @sink(cap-two-a)
	}, func() {
		sys.Run(b) // @sink(none)
	})
}

// capNever: the closure is made and never called. A rule that takes the closure to run where it is made
// (C-7) reports this line: it is the limit of the rule (what it does not see is not called, or
// is called by code that is not seen). It is quiet now (the sink in the closure is `@sink(none)`); the
// sink after it is there so that the source has a flow.
func capNever(c *web.Context) {
	a := c.Query("never") // @source(cap-never)
	_ = func() {
		sys.Run(a) // @sink(none)
	}
	sys.Run(a) // @sink(cap-never)
}

// mkRun returns a closure that holds its argument: the closure is made here, and called by whoever
// gets it.
func mkRun(s string) func() {
	return func() {
		sys.Run(s) // @sink(cap-returned)
	}
}

// capReturned: a closure returned by a function, called by the caller. The captured value is a
// parameter of the function that made the closure.
func capReturned(c *web.Context) {
	a := c.Query("returned") // @source(cap-returned) @miss(silent)
	f := mkRun(a)
	f()
}

// capLate: the variable is captured by reference: it is tainted after the closure is made, before the
// helper calls it.
func capLate(c *web.Context) {
	var x string
	f := func() {
		sys.Run(x) // @sink(cap-late)
	}
	x = c.Query("late") // @source(cap-late) @miss(silent)
	runCb(f)
}

// capHandler: the parameter of the closure is a source (the shape of a registered handler), and the
// closure writes it into a variable it captured; the variable is used somewhere else. The write is
// "what the function did", and it is not the taint of a source written back to the callers (C-1,
// ADR-048).
func capHandler(c *web.Context, g *web.RouterGroup) {
	var last string
	g.GET("/x", func(c *web.Context) {
		last = c.Query("handler") // @source(cap-handler) @miss(silent)
	})
	runCb(func() {
		sys.Run(last) // @sink(cap-handler)
	})
}

// capSeenAndPassed: the closure is called where it is made, and also given to a helper. The call that is
// seen finds it; it is one finding.
func capSeenAndPassed(c *web.Context) {
	a := c.Query("seen") // @source(cap-seen)
	f := func() {
		sys.Run(a) // @sink(cap-seen)
	}
	f()
	runCb(f)
}

// capFuncCall: the captured value is a function, and the closure calls it with a value of the
// caller (PARAM-CALL: the call of a function that the closure captured is not resolved).
func capFuncCall(c *web.Context) {
	a := c.Query("funccall") // @source(cap-funccall) @miss(silent)
	g := func(s string) {
		sys.Run(s) // @sink(cap-funccall)
	}
	runCb(func() {
		g(a)
	})
}

// capSanitized: the closure cleans what it captured before the sink; and the sink that the closure
// does not reach is the one that has the flow.
func capSanitized(c *web.Context) {
	a := c.Query("sanitized") // @source(cap-sanitized)
	runCb(func() {
		sys.Run(valid.Quote(a)) // @sink(none)
	})
	sys.Run(a) // @sink(cap-sanitized)
}
