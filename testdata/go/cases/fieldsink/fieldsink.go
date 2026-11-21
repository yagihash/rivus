// Package fieldsink holds sinks that name a part of their value (docs/DESIGN-field-sinks.md): the Q of
// a Req is dangerous, the Meta is not. A sink that is reached by the taint of Meta only is quiet; one that
// may be reached through Q, without the field being told, is reported with field_imprecision.
package fieldsink

import "example.com/cases/lib"

// --- one function ---

func Direct() {
	r := &lib.Req{}
	r.Q = lib.Source() // @source(direct)
	lib.SinkReq(r)     // @sink(direct)
}

func DirectDecoy() {
	r := &lib.Req{}
	r.Meta = lib.Source()
	lib.SinkReq(r)
}

func Literal() {
	r := &lib.Req{Q: lib.Source(), Meta: "constant"} // @source(literal)
	lib.SinkReq(r)                                   // @sink(literal)
}

func LiteralDecoy() {
	r := &lib.Req{Q: "constant", Meta: lib.Source()}
	lib.SinkReq(r)
}

// --- the structure goes through other functions: the summary says which part is read ---

func use(r *lib.Req) {
	lib.SinkReq(r) // @sink(viacall)
}

func ViaCall() {
	r := &lib.Req{}
	r.Q = lib.Source() // @source(viacall)
	use(r)
}

func useDecoy(r *lib.Req) {
	lib.SinkReq(r)
}

func ViaCallDecoy() {
	r := &lib.Req{}
	r.Meta = lib.Source()
	useDecoy(r)
}

func relay(r *lib.Req) { use2(r) }

func use2(r *lib.Req) {
	lib.SinkReq(r) // @sink(twolevels)
}

func TwoLevels() {
	r := &lib.Req{Q: lib.Source()} // @source(twolevels)
	relay(r)
}

func relayDecoy(r *lib.Req) { use2Decoy(r) }

func use2Decoy(r *lib.Req) {
	lib.SinkReq(r)
}

func TwoLevelsDecoy() {
	r := &lib.Req{Meta: lib.Source()}
	relayDecoy(r)
}

// a parameter that is the taint (the part that the caller gives is what counts)
func Param(q string, m string) {
	lib.SinkReq(&lib.Req{Q: q, Meta: m})
}

// --- copies ---

func CopyWhole() {
	r := &lib.Req{Q: lib.Source()} // @source(copy)
	var c lib.Req
	c = *r
	lib.SinkReq(&c) // @sink(copy)
}

func CopyWholeDecoy() {
	r := &lib.Req{Meta: lib.Source()}
	var c lib.Req
	c = *r
	lib.SinkReq(&c)
}

// --- a wrapper: the path has more steps ---

func Wrapped() {
	w := &lib.Wrap{Req: &lib.Req{Q: lib.Source()}} // @source(wrapped)
	lib.SinkWrap(w)                                // @sink(wrapped)
}

func WrappedDecoyMeta() {
	w := &lib.Wrap{Req: &lib.Req{Meta: lib.Source()}}
	lib.SinkWrap(w)
}

func WrappedDecoyBeside() {
	w := &lib.Wrap{Req: &lib.Req{}, N: lib.Source()}
	lib.SinkWrap(w)
}

// --- the whole value is tainted: the field is not told (field_imprecision) ---

func Whole() {
	r := lib.NewReq(lib.Source()) // @source(whole)
	lib.SinkReq(r)                // @sink(whole)
}

func useWhole(r *lib.Req) {
	lib.SinkReq(r) // @sink(wholecall)
}

func WholeThroughCall() {
	useWhole(lib.NewReq(lib.Source())) // @source(wholecall)
}

// the same flow, told and not told: the finding is the one that tells the field, with no field_imprecision
func SureWins() {
	s := lib.Source() // @source(surewins)
	r := lib.NewReq(s)
	r.Q = s
	lib.SinkReq(r) // @sink(surewins)
}

// --- a function that is not read: it may move the taint into the field ---

func Opaque() {
	r := &lib.Req{}
	r.Meta = lib.Source() // @source(opaque)
	lib.Opaque(r)
	lib.SinkReq(r) // @sink(opaque)
}

func OpaqueOther() {
	r := &lib.Req{}
	r.Meta = lib.Source()
	lib.Opaque(&lib.Req{})
	lib.SinkReq(r)
}

func opaqueInside(r *lib.Req) {
	lib.Opaque(r)
	lib.SinkReq(r) // @sink(opaqueinside)
}

func OpaqueInside() {
	r := &lib.Req{}
	r.Meta = lib.Source() // @source(opaqueinside)
	opaqueInside(r)
}

// the summary of a function that has both ways to the sink (told, and not told: a function that is not read
// had the value): a caller that puts the taint in the field gets the finding with no field_imprecision
func bothWays(r *lib.Req) {
	lib.Opaque(r)
	lib.SinkReq(r) // @sink(surecall)
}

func SureThroughCall() {
	r := &lib.Req{Q: lib.Source()} // @source(surecall)
	bothWays(r)
}

// --- a name that the program does not have: the sink is read whole, and says so ---

func Misspelled() {
	r := &lib.Req{}
	r.Meta = lib.Source() // @source(misspelled)
	lib.SinkReqTypo(r)    // @sink(misspelled)
}

// --- through an interface of the project whose implementation has the rule (ADR-068) ---

type runner interface{ Run(r *lib.Req) }

func run(q runner, r *lib.Req) {
	q.Run(r) // @sink(viaiface)
}

func ViaInterface(s *lib.Store) {
	r := &lib.Req{Q: lib.Source()} // @source(viaiface)
	run(s, r)
}

func runDecoy(q runner, r *lib.Req) {
	q.Run(r)
}

func ViaInterfaceDecoy(s *lib.Store) {
	r := &lib.Req{Meta: lib.Source()}
	runDecoy(s, r)
}

// the receiver of the implementation holds the request
func Method(s *lib.Store) {
	r := &lib.Req{Q: lib.Source()} // @source(method)
	s.Run(r)                       // @sink(method)
}

// --- the address of a variable goes to a function that is not read, and the value of the variable goes to a sink
// that takes it by value (the shape of every Cloud Spanner call). The mark that a function that is not read had the
// value is on the variable (the memory), and a read of the variable by value carries it. ---

func ValAfterPtr() {
	r := lib.Req{Meta: lib.Source()} // @source(valafterptr)
	lib.Opaque(&r)
	lib.SinkReqVal(r) // @sink(valafterptr)
}

func ValAfterPtrAssign() {
	r := lib.Req{}
	r.Meta = lib.Source() // @source(valafterptrassign)
	lib.Opaque(&r)
	lib.SinkReqVal(r) // @sink(valafterptrassign)
}

func ValAfterPtrVar() {
	r := lib.Req{Meta: lib.Source()} // @source(valafterptrvar)
	p := &r
	lib.Opaque(p)
	lib.SinkReqVal(*p) // @sink(valafterptrvar)
}

func moveReq(r *lib.Req) { lib.Opaque(r) }

func ValAfterPtrInside() {
	r := lib.Req{Meta: lib.Source()} // @source(valafterptrinside)
	moveReq(&r)
	lib.SinkReqVal(r) // @sink(valafterptrinside)
}

func ValAfterPtrNew() {
	r := new(lib.Req)
	r.Meta = lib.Source() // @source(valafterptrnew)
	lib.Opaque(r)
	lib.SinkReqVal(*r) // @sink(valafterptrnew)
}

// a value that is given and returned
func ValRewrite() {
	r := lib.Req{Meta: lib.Source()} // @source(valrewrite)
	r = lib.OpaqueRewrite(r)
	lib.SinkReqVal(r) // @sink(valrewrite)
}

// decoys: nothing that is not read had the variable, or it had another one
func ValAfterPtrDecoy() {
	r := lib.Req{Meta: lib.Source()}
	lib.SinkReqVal(r)
}

func ValAfterPtrOther() {
	r := lib.Req{Meta: lib.Source()}
	o := lib.Req{}
	lib.Opaque(&o)
	lib.SinkReqVal(r)
}

// --- the mark does not depend on the order of the instructions: a sink that was evaluated before the mark was made
// is evaluated again when the mark is made (a loop, a deferred sink, a copy that was made before) ---

func LoopMark() {
	r := lib.Req{Meta: lib.Source()} // @source(loopmark)
	for i := 0; i < 2; i++ {
		lib.SinkReqVal(r) // @sink(loopmark)
		lib.Opaque(&r)
	}
}

func DeferMark() {
	r := &lib.Req{Meta: lib.Source()} // @source(defermark)
	defer lib.SinkReq(r)              // @sink(defermark)
	lib.Opaque(r)
}

func CopyBeforeMark() {
	r := lib.Req{Meta: lib.Source()} // @source(copybeforemark)
	r2 := r
	lib.Opaque(&r)
	lib.SinkReqVal(r2) // @sink(copybeforemark)
}
