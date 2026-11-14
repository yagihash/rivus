package sites

import (
	"net/http"
	"os/exec"

	"example.com/cases/lib"
)

func Literal() {
	lib.Sink("constant") // @literal
}

func Dynamic() {
	s := lib.Source() // @src
	lib.Sink(s)       // @dyn
}

func TwoSinks() {
	lib.Sink("a")
	lib.Sink("b")
}

func Exec(name string) {
	_ = exec.Command("ls", "-l") // @lit-exec
	_ = exec.Command(name, "-l") // @dyn-exec
	_ = exec.Command("ls", name) // @dyn-exec2
}

func Handler(w http.ResponseWriter, r *http.Request) {
	_ = r
}
