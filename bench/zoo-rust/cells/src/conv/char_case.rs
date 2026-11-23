// @cell(F/conv/char-case)
// @shape: every char of the text is read by chars(), converted with to_ascii_uppercase and written with String::push
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-case/vuln", vuln);
    m::case("F/conv/char-case/fixed", fixed);
    m::case("F/conv/char-case/decoy", decoy);
}

fn f(s: &str) -> String {
    let mut out = String::new();
    for c in s.chars() {
        out.push(c.to_ascii_uppercase());
    }
    out
}

fn vuln() { // @vuln
    let v = m::src("conv-char-case-vuln"); // @source(conv-char-case-vuln) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f(&v)); // @sink(conv-char-case-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-case-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-case-decoy"); // @source(conv-char-case-decoy) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f("constant")); // @sink(none) @nosink(conv-char-case-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-case-decoy)
}
