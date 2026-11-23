// @cell(F/conv/char-index)
// @shape: the first byte is read with as_bytes()[0] and made a char, and the rest is a slice of the text (&s[1..])
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-index/vuln", vuln);
    m::case("F/conv/char-index/fixed", fixed);
    m::case("F/conv/char-index/decoy", decoy);
}

fn f(s: &str) -> String {
    let mut out = String::new();
    out.push(s.as_bytes()[0] as char);
    out.push_str(&s[1..]);
    out
}

fn vuln() { // @vuln
    let v = m::src("conv-char-index-vuln"); // @source(conv-char-index-vuln)
    m::sink_cmd(&f(&v)); // @sink(conv-char-index-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-index-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-index-decoy"); // @source(conv-char-index-decoy)
    m::sink_cmd(&f("constant")); // @sink(none) @nosink(conv-char-index-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-index-decoy)
}
