package taint

import "testing"

// The names of the implementations of a trait method of Rust are cut apart to find the ones a row with a method calls
// (`Write::write_all` calls `write`).
func TestSplitImplID(t *testing.T) {
	cases := []struct {
		id, trait, method, self string
		ok                      bool
	}{
		{"<zoo::ShellWriter as std::io::Write>::write", "std::io::Write", "write", "zoo::ShellWriter", true},
		{"<zoo::W<u8> as std::io::Write>::write", "std::io::Write", "write", "zoo::W", true},
		{"<std::vec::Vec<u8> as core::convert::From<&str>>::from", "core::convert::From", "from", "std::vec::Vec", true},
		{"<zoo::A as zoo::Tr>::m::<T>", "zoo::Tr", "m", "zoo::A", true},
		{"zoo::free_function", "", "", "", false},
		{"<zoo::NoTrait>::m", "", "", "", false},
	}
	for _, c := range cases {
		trait, method, ok := splitImplID(c.id)
		if ok != c.ok || trait != c.trait || method != c.method {
			t.Errorf("%s: got %q %q %v, want %q %q %v", c.id, trait, method, ok, c.trait, c.method, c.ok)
		}
		if c.ok && implSelfType(c.id) != c.self {
			t.Errorf("%s: self type %q, want %q", c.id, implSelfType(c.id), c.self)
		}
	}
}

func TestSameTraitPath(t *testing.T) {
	same := [][2]string{
		{"std::io::Write", "std::io::Write"},
		{"core::iter::traits::iterator::Iterator", "std::iter::Iterator"},
		{"zoo::a::Tr", "zoo::a::Tr"},
	}
	for _, c := range same {
		if !sameTraitPath(c[0], c[1]) {
			t.Errorf("%s and %s are one trait", c[0], c[1])
		}
	}
	other := [][2]string{
		{"std::io::Write", "std::fmt::Write"},
		{"zoo::a::Tr", "zoo::b::Tr"},
		{"std::io::Write", "zoo::io::Write"},
	}
	for _, c := range other {
		if sameTraitPath(c[0], c[1]) {
			t.Errorf("%s and %s are two traits", c[0], c[1])
		}
	}
}
