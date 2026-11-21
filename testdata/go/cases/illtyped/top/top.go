// Package top imports the package that imports the one with the type error (a second step).
package top

import "example.com/illtyped/user"

func Main() {
	user.Run()
	_ = user.Quiet()
}
