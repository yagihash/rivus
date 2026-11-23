package numrune

import "example.com/cases/lib"

// PerRune reads the text one rune at a time: the rune is a number, so the data stops there.
func PerRune(s string) []byte {
	var out []byte
	for _, r := range s {
		out = append(out, byte(r))
	}
	return out
}

func Use() {
	lib.Sink(string(PerRune(lib.Source())))
}
