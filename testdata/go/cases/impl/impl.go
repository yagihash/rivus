package impl

import "example.com/cases/lib"

// Store is the interface the sink rule is written for. Its implementations
// are sinks too, even when they are called directly.
type Store interface{ Save(s string) }

type fileStore struct{}

func (fileStore) Save(s string) { _ = s }

type memStore struct{ n int }

func (m *memStore) Save(s string) { m.n++ }

func ConcreteValueReceiver() {
	s := lib.Source() // @source(implv)
	var f fileStore
	f.Save(s) // @sink(implv)
}

func ConcretePointerReceiver() {
	s := lib.Source() // @source(implp)
	m := &memStore{}
	m.Save(s) // @sink(implp)
}

func ThroughTheInterface(st Store) {
	s := lib.Source() // @source(impli)
	st.Save(s)        // @sink(impli)
}

func OnlyConstants() {
	var f fileStore
	f.Save("constant") // @sink(none)
}
