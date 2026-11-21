// Package fine does not depend on the package with the type error: it is built, and its flow is found.
package fine

import "example.com/illtyped/lib"

func Direct() {
	lib.Sink(lib.Source())
}
