// Package lib provides the sources, sinks and sanitizers used by the test
// cases. The functions are modeled by testdata/go/rules.yaml; the bodies are
// never analysed because lib is not part of the loaded patterns.
package lib

// Source returns attacker controlled data.
func Source() string { return "tainted" }

// SourceBytes returns attacker controlled bytes.
func SourceBytes() []byte { return []byte("tainted") }

// Sink must not receive attacker controlled data.
func Sink(s string) {}

// SinkBytes must not receive attacker controlled data.
func SinkBytes(b []byte) {}

// SinkAny must not receive attacker controlled data.
func SinkAny(v any) {}

// Sanitize removes the taint.
func Sanitize(s string) string { return s }

// Pair is a struct with two fields.
type Pair struct {
	A string
	B string
}

// Runner is an interface used for dispatch tests.
type Runner interface {
	Run(s string)
}

// Fill writes attacker controlled data through the pointer, as a database read
// does (rows.Scan(&x)).
func Fill(dst *string) error { return nil }

// FillAny is Fill for a destination given as any (db.Find(&rows)).
func FillAny(dst any) error { return nil }

// StoreRead reads from the data store; nothing is followed through it.
func StoreRead() string { return "" }

// Open is a sink of its argument that returns a value built from it (os.Open,
// http.Get): the rules say where the argument must not arrive, not what the
// result holds.
func Open(path string) string { return path }

// DB has a method that is a sink and returns a value (the (*sql.DB).Prepare shape).
type DB struct{}

// Prepare is a sink of the query; the statement it returns holds it.
func (*DB) Prepare(query string) string { return query }

// External stands for the output of an external program: the model says that the argument may come out, as a guess.
func External(s string) string { return s }

// Exact is the same with a flow that is not a guess.
func Exact(s string) string { return s }

// Apply calls f with s and returns what it returns (the model has calls).
func Apply(s string, f func(string) string) string { return f(s) }

// Each calls f with every element of xs (the model has calls, and nothing flows).
func Each(xs []string, f func(string)) {
	for _, x := range xs {
		f(x)
	}
}

// Later runs f on its own (calls with mode go).
func Later(f func()) { go f() }

// Req is a request with a text that is dangerous (Q) and one that is not (Meta): the shape of the
// requests of a generated API (testdata/go/cases/fieldsink).
type Req struct {
	Q    string
	Meta string
}

// Wrap holds a request, and a text beside it.
type Wrap struct {
	Req *Req
	N   string
}

// SinkReq runs the request: the Q of it is the sink.
func SinkReq(r *Req) {}

// SinkWrap runs the request of the wrapper: the Q of the Req is the sink.
func SinkWrap(w *Wrap) {}

// SinkReqTypo is a sink of a field that Req does not have (a misspelling in the rule).
func SinkReqTypo(r *Req) {}

// NewReq makes a request of a text (the model says the whole request holds it).
func NewReq(q string) *Req { return nil }

// Opaque is a function that has no model: the analysis does not know what it does to the request.
func Opaque(r *Req) {}

// Store runs requests: the Q of the request is the sink.
type Store struct{}

// Run runs the request.
func (*Store) Run(r *Req) {}

// SinkReqVal runs the request that it is given by value: the Q of it is the sink (the shape of every Cloud Spanner
// call, which takes the Statement by value).
func SinkReqVal(r Req) {}

// OpaqueRewrite is a function that has no model and takes the request by value and returns it.
func OpaqueRewrite(r Req) Req { return r }

// ReadInto reads into the variables that it is given (the model sends the first argument to them).
func ReadInto(src string, dst ...*string) {}
