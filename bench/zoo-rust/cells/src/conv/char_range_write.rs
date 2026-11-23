// @cell(F/conv/char-range-write)
// @shape: every char of the text is read by a for loop over chars() and written with String::push
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-range-write/vuln", vuln);
    m::case("F/conv/char-range-write/fixed", fixed);
    m::case("F/conv/char-range-write/decoy", decoy);
}

fn f(s: &str) -> String {
    let mut out = String::new();
    for c in s.chars() {
        out.push(c);
    }
    out
}

fn vuln() { // @vuln
    let v = m::src("conv-char-range-write-vuln"); // @source(conv-char-range-write-vuln) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f(&v)); // @sink(conv-char-range-write-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-range-write-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-range-write-decoy"); // @source(conv-char-range-write-decoy) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f("constant")); // @sink(none) @nosink(conv-char-range-write-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-range-write-decoy)
}
