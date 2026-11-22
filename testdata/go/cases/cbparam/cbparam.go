package cbparam

// Route registers a handler: the handler is called with the request and then the segments of the path.
func Route(path string, h func(req string, id string, extra string)) {}

type Router struct{}

// Get is a method: the receiver is not counted in the position of the rules.
func (r *Router) Get(path string, h func(req string, id string)) {}

func Literal() {
	Route("/a/<id>", func(req string, id string, extra string) {})
}

func named(req string, id string, extra string) {}

func Named() {
	Route("/b/<id>", named)
}

func ThroughLocal() {
	h := func(req string, id string, extra string) {}
	Route("/c/<id>", h)
}

func Registers(h func(req string, id string, extra string)) {
	Route("/d/<id>", h)
}

func Chosen(f bool) {
	h := named
	if f {
		h = func(req string, id string, extra string) {}
	}
	Route("/e/<id>", h)
}

func OnMethod() {
	r := &Router{}
	r.Get("/f/<id>", func(req string, id string) {})
}
