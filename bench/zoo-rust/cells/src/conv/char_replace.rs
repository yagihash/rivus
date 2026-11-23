// @cell(F/conv/char-replace)
// @shape: every char of the text is read by chars(), mapped one by one (a replacement) and collected with collect::<String>()
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-replace/vuln", vuln);
    m::case("F/conv/char-replace/fixed", fixed);
    m::case("F/conv/char-replace/decoy", decoy);
}

fn f(s: &str) -> String {
    s.chars().map(|c| if c == 'S' { 's' } else { c }).collect::<String>()
}

fn vuln() { // @vuln
    let v = m::src("conv-char-replace-vuln"); // @source(conv-char-replace-vuln)
    m::sink_cmd(&f(&v)); // @sink(conv-char-replace-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-replace-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-replace-decoy"); // @source(conv-char-replace-decoy)
    m::sink_cmd(&f("constant")); // @sink(none) @nosink(conv-char-replace-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-replace-decoy)
}
