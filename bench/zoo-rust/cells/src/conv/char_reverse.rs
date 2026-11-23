// @cell(F/conv/char-reverse)
// @shape: the text is reversed char by char (chars().rev().collect::<String>()), and reversed once more
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-reverse/vuln", vuln);
    m::case("F/conv/char-reverse/fixed", fixed);
    m::case("F/conv/char-reverse/decoy", decoy);
}

fn reverse(s: &str) -> String {
    s.chars().rev().collect::<String>()
}

fn f(s: &str) -> String {
    reverse(&reverse(s))
}

fn vuln() { // @vuln
    let v = m::src("conv-char-reverse-vuln"); // @source(conv-char-reverse-vuln)
    m::sink_cmd(&f(&v)); // @sink(conv-char-reverse-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-reverse-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-reverse-decoy"); // @source(conv-char-reverse-decoy)
    m::sink_cmd(&f("constant")); // @sink(none) @nosink(conv-char-reverse-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-reverse-decoy)
}
