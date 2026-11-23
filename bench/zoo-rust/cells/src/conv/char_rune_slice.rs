// @cell(F/conv/char-rune-slice)
// @shape: the text is collected whole into a Vec<char>, sliced, and collected back whole into Strings (no element is read)
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-rune-slice/vuln", vuln);
    m::case("F/conv/char-rune-slice/fixed", fixed);
    m::case("F/conv/char-rune-slice/decoy", decoy);
}

fn f(s: &str) -> String {
    let cs: Vec<char> = s.chars().collect();
    let head: String = cs[..1].iter().collect();
    let tail: String = cs[1..].iter().collect();
    head + &tail
}

fn vuln() { // @vuln
    let v = m::src("conv-char-rune-slice-vuln"); // @source(conv-char-rune-slice-vuln)
    m::sink_cmd(&f(&v)); // @sink(conv-char-rune-slice-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-rune-slice-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-rune-slice-decoy"); // @source(conv-char-rune-slice-decoy)
    m::sink_cmd(&f("constant")); // @sink(none) @nosink(conv-char-rune-slice-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-rune-slice-decoy)
}
