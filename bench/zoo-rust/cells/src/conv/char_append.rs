// @cell(F/conv/char-append)
// @shape: the bytes of the text are read one by one (bytes()) and pushed to a Vec<u8> that String::from_utf8 turns back into a string
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-append/vuln", vuln);
    m::case("F/conv/char-append/fixed", fixed);
    m::case("F/conv/char-append/decoy", decoy);
}

fn f(s: &str) -> String {
    let mut out: Vec<u8> = Vec::new();
    for b in s.bytes() {
        out.push(b);
    }
    String::from_utf8(out).unwrap()
}

fn vuln() { // @vuln
    let v = m::src("conv-char-append-vuln"); // @source(conv-char-append-vuln) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f(&v)); // @sink(conv-char-append-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-append-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-append-decoy"); // @source(conv-char-append-decoy) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f("constant")); // @sink(none) @nosink(conv-char-append-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-append-decoy)
}
