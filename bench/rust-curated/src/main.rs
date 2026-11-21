// The start of the program (a binary beside the library, as the programs that are read are: parseable has a main.rs and a lib.rs).
// A program that starts at `main` is read as a closed world: what the library has that is `pub` is called by the code that
// is read, and nothing outside it (docs/DESIGN-capture.md). The library alone, with no `main`, is the open case: bench/rust-curated-lib.
fn main() {}
