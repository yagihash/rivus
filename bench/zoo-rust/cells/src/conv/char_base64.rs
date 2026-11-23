// @cell(F/conv/char-base64)
// @shape: a home-made base64 encoder (three bytes of as_bytes() make four symbols) and decoder, with padding
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-base64/vuln", vuln);
    m::case("F/conv/char-base64/fixed", fixed);
    m::case("F/conv/char-base64/decoy", decoy);
}

const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

fn encode(s: &str) -> String {
    let u = s.as_bytes();
    let mut out = Vec::new();
    let mut i = 0;
    while i < u.len() {
        let mut n: u32 = 0;
        let mut k = 0;
        for j in 0..3 {
            n <<= 8;
            if i + j < u.len() {
                n |= u[i + j] as u32;
                k += 1;
            }
        }
        out.push(ALPHABET[((n >> 18) & 63) as usize]);
        out.push(ALPHABET[((n >> 12) & 63) as usize]);
        out.push(if k > 1 { ALPHABET[((n >> 6) & 63) as usize] } else { b'=' });
        out.push(if k > 2 { ALPHABET[(n & 63) as usize] } else { b'=' });
        i += 3;
    }
    String::from_utf8(out).unwrap()
}

fn decode(s: &str) -> String {
    let u = s.as_bytes();
    let mut out = Vec::new();
    let mut i = 0;
    while i + 3 < u.len() {
        let mut n: u32 = 0;
        let mut pad = 0;
        for j in 0..4 {
            n <<= 6;
            if u[i + j] == b'=' {
                pad += 1;
            } else {
                n |= ALPHABET.iter().position(|&a| a == u[i + j]).unwrap() as u32;
            }
        }
        out.push((n >> 16) as u8);
        if pad < 2 {
            out.push((n >> 8) as u8);
        }
        if pad < 1 {
            out.push(n as u8);
        }
        i += 4;
    }
    String::from_utf8(out).unwrap()
}

fn f(s: &str) -> String {
    decode(&encode(s))
}

fn vuln() { // @vuln
    let v = m::src("conv-char-base64-vuln"); // @source(conv-char-base64-vuln) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f(&v)); // @sink(conv-char-base64-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-base64-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-base64-decoy"); // @source(conv-char-base64-decoy) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f("constant")); // @sink(none) @nosink(conv-char-base64-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-base64-decoy)
}
