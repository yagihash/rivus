// @cell(F/conv/char-range-append)
// @shape: every character of the text is read by a loop over `chars()` and `c as u8` is pushed to a Vec<u8>, which `String::from_utf8` turns back whole
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-range-append/vuln", vuln);
    m::case("F/conv/char-range-append/fixed", fixed);
    m::case("F/conv/char-range-append/decoy", decoy);
}

fn f(s: &str) -> String {
    let mut out: Vec<u8> = Vec::new();
    for c in s.chars() {
        out.push(c as u8);
    }
    String::from_utf8(out).unwrap()
}

fn vuln() { // @vuln
    let v = m::src("conv-char-range-append-vuln"); // @source(conv-char-range-append-vuln) @miss(unmodeled_library_call) @ref(#185)
    m::sink_sql(&f(&v)); // @sink(conv-char-range-append-vuln)
}

fn fixed() { // @fixed
    m::sink_sql(&f(&m::clean_sql(&m::src("conv-char-range-append-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-range-append-decoy"); // @source(conv-char-range-append-decoy) @miss(unmodeled_library_call) @ref(#185)
    m::sink_sql(&f("constant")); // @sink(none) @nosink(conv-char-range-append-decoy)
    m::sink_cmd(&f(&a)); // @sink(conv-char-range-append-decoy)
}
