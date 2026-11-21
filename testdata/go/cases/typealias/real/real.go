package real

type A struct{ P string }

type B struct{ P string }

type C struct{ F string }

type D struct{ F string }

type E struct{ P string }

type F struct{ P string }

type G struct{ P string }

type H struct{ P string }

// Box is a generic type: its type argument is where an alias can stand inside a type.
type Box[T any] struct{ V T }
