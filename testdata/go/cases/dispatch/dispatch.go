package dispatch

import "example.com/cases/lib"

// An interface call is a call to every implementation in the program. When
// the caller made the interface value right there, its type is known: the flow
// goes through the implementation of that type only.

// --- the type is known at the call, and it is not the one with the sink ---

type Backend interface{ Fetch(key string) }

type memBackend struct{}

func (*memBackend) Fetch(key string) {}

type diskBackend struct{}

func (diskBackend) Fetch(key string) {
	lib.Sink(key) // @sink(dispatch_disk)
}

func get(b Backend, key string) { b.Fetch(key) }

// no finding: only memBackend is passed here
func Serve() {
	key := lib.Source() // @source(dispatch_mem)
	get(&memBackend{}, key)
}

// the same function, with the other type: the flow is real
func CLI() {
	key := lib.Source() // @source(dispatch_disk)
	get(diskBackend{}, key)
}

// --- the type is not known at the call: every implementation stays ---

type Store interface{ Put(key string) }

type memStore struct{}

func (*memStore) Put(key string) {}

type fileStore struct{}

func (fileStore) Put(key string) {
	lib.Sink(key) // @sink(dispatch_unknown)
}

func pick() Store { return fileStore{} }

func put(s Store, key string) { s.Put(key) }

func Unknown() {
	key := lib.Source() // @source(dispatch_unknown)
	put(pick(), key)
}

// --- a method promoted from an embedded field: the flow is real ---

type Sink2 interface{ Write(key string) }

type plain struct{}

func (plain) Write(key string) {}

type inner struct{}

func (inner) Write(key string) {
	lib.Sink(key) // @sink(dispatch_embedded)
}

type outer struct{ inner }

func write(w Sink2, key string) { w.Write(key) }

func Embedded() {
	key := lib.Source() // @source(dispatch_embedded)
	write(outer{}, key)
}

// --- the receiver's fields, through an interface (CORE-J) ---
// A method with a pointer receiver reads a field of the receiver; the value
// reached the interface as a pointer.

type Getter interface{ Get() string }

type ptrRecv struct{ f string }

func (t *ptrRecv) Get() string { return t.f }

type valRecv struct{ f string }

func (t valRecv) Get() string { return t.f }

// pointer receiver, the field read in the method, the result used at the sink
func PtrRecv() {
	var g Getter = &ptrRecv{f: lib.Source()} // @source(dispatch_ptr_recv)
	lib.Sink(g.Get())                        // @sink(dispatch_ptr_recv)
}

// value receiver: no regression
func ValRecv() {
	var g Getter = valRecv{f: lib.Source()} // @source(dispatch_val_recv)
	lib.Sink(g.Get())                       // @sink(dispatch_val_recv)
}

// the sink is in the method
type Runner interface{ Run() }

type cmdRunner struct{ cmd string }

func (c *cmdRunner) Run() {
	lib.Sink(c.cmd) // @sink(dispatch_sink_in_method)
}

func run(r Runner) { r.Run() }

func SinkInMethod() {
	run(&cmdRunner{cmd: lib.Source()}) // @source(dispatch_sink_in_method)
}

// a method value
func MethodValue() {
	var g Getter = &ptrRecv{f: lib.Source()} // @source(dispatch_method_value)
	h := g.Get
	lib.Sink(h()) // @sink(dispatch_method_value)
}

// a package variable of the interface type
var globalGetter Getter

func SetGlobal() {
	globalGetter = &ptrRecv{f: lib.Source()} // @source(dispatch_global_iface)
}

func ReadGlobal() {
	lib.Sink(globalGetter.Get()) // @sink(dispatch_global_iface)
}
