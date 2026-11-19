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
