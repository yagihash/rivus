package taint

import "testing"

// A receiver of a standard type that holds no writer (a `Vec<u8>`) is none of the program's implementations; one that may
// delegate to what it holds (`Box<W>`: the type arguments are not in the type) or is not known to be a leaf stays a candidate.
func TestExcludesStdTypes(t *testing.T) {
	d := &dispatchSpec{recv: "&mut app::ShellWriter", all: []string{"&mut app::ShellWriter", "&mut app::OtherWriter"}}
	for _, c := range []string{"&mut std::vec::Vec", "std::vec::Vec", "&mut alloc::string::String", "std::fs::File", "&mut std::io::stdio::Stdout"} {
		if !d.excludes(c) {
			t.Errorf("%s: a leaf of std is none of the program's writers, but it is not excluded", c)
		}
	}
	for _, c := range []string{"", "&mut std::boxed::Box", "alloc::boxed::Box", "&mut alloc::boxed::Box", "std::rc::Rc", "alloc::sync::Arc",
		"std::cell::RefCell", "std::sync::Mutex", "std::io::BufWriter", "std::io::LineWriter", "std::some::Unknown", "&mut app::ShellWriter"} {
		if d.excludes(c) {
			t.Errorf("%q: a type that may delegate (or is not known) must stay a candidate", c)
		}
	}
	if !d.excludes("&mut app::OtherWriter") {
		t.Error("another implementation of the call is not excluded")
	}
}
