// @cell(F/conv/char-hex)
// @shape: a home-made hex encoder (two table lookups per byte of bytes()) and decoder (two digits make a byte)
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-hex/vuln", vuln);
    m::case("F/conv/char-hex/fixed", fixed);
    m::case("F/conv/char-hex/decoy", decoy);
}

const DIGITS: &[u8] = b"0123456789abcdef";

fn encode(s: &str) -> String {
    let mut out = Vec::new();
    for b in s.bytes() {
        out.push(DIGITS[(b >> 4) as usize]);
        out.push(DIGITS[(b & 15) as usize]);
    }
    String::from_utf8(out).unwrap()
}

fn unhex(c: u8) -> u8 {
    if c >= b'a' { c - b'a' + 10 } else { c - b'0' }
}

fn decode(h: &str) -> String {
    let u = h.as_bytes();
    let mut out = Vec::new();
    let mut i = 0;
    while i + 1 < u.len() {
        out.push((unhex(u[i]) << 4) | unhex(u[i + 1]));
        i += 2;
    }
    String::from_utf8(out).unwrap()
}

fn f(s: &str) -> String {
    decode(&encode(s))
}

fn vuln() { // @vuln
    let v = m::src("conv-char-hex-vuln"); // @source(conv-char-hex-vuln) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f(&v)); // @sink(conv-char-hex-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-hex-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-hex-decoy"); // @source(conv-char-hex-decoy) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f("constant")); // @sink(none) @nosink(conv-char-hex-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-hex-decoy)
}
