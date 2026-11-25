package cbparam

// The helpers that register what they are given: the function is what the callers pass.

type handler = func(req string, id string, extra string)

func inner2(h handler) { Route("/g/<id>", h) }

func outer2(h handler) { inner2(h) }

func named2(req string, id string, extra string) {}

// TwoLevels: a literal and a named function, through two helpers.
func TwoLevels() {
	outer2(func(req string, id string, extra string) {})
	outer2(named2)
}

// recursive: the helper calls itself with what it is given.
func recursive(n int, h handler) {
	if n > 0 {
		recursive(n-1, h)
		return
	}
	Route("/h/<id>", h)
}

func Recursion() {
	recursive(3, func(req string, id string, extra string) {})
}

// maybe registers on one branch only: the function it is given is still one that it may register.
func maybe(on bool, h handler) {
	if on {
		Route("/i/<id>", h)
	}
}

func Maybe(on bool) {
	maybe(on, func(req string, id string, extra string) {})
}

// skips never registers what it is given.
func skips(h handler) { _ = h }

func Skips() {
	skips(func(req string, id string, extra string) {})
}

// viaValue calls the helper through a function value: the first operand of the call is the value.
var register = outer2

func ViaValue() {
	register(func(req string, id string, extra string) {})
}

// twice: two helpers that are called with the same literal are one target
func twice(h handler) { Route("/j/<id>", h) }

func Twice() {
	h := func(req string, id string, extra string) {}
	twice(h)
	twice(h)
}

// Chosen2: one caller that gives a variable of two functions: that caller is said, the other is marked.
func chosenHelper(h handler) { Route("/k/<id>", h) }

func ChosenCaller(f bool) {
	h := named2
	if f {
		h = func(req string, id string, extra string) {}
	}
	chosenHelper(h)
}

func LiteralCaller() {
	chosenHelper(func(req string, id string, extra string) {})
}
