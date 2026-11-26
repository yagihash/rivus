// @cell(F/conv/char-decode-rune)
// @shape: a buffer type reads the text one character at a time in a loop (the character at the index, then its length in bytes), pushes `c as u8` to a Vec<u8> field, and gives the text back by slicing the field
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-decode-rune/vuln", vuln);
    m::case("F/conv/char-decode-rune/fixed", fixed);
    m::case("F/conv/char-decode-rune/decoy", decoy);
}

struct Buffer {
    arr: Vec<u8>,
}

fn new_buffer(data: &str) -> Buffer {
    let mut buf: Vec<u8> = Vec::with_capacity(data.len());
    let mut idx = 0;
    while idx < data.len() {
        let c = data[idx..].chars().next().unwrap();
        idx += c.len_utf8();
        buf.push(c as u8);
    }
    Buffer { arr: buf }
}

impl Buffer {
    fn slice(&self, i: usize, j: usize) -> String {
        String::from_utf8(self.arr[i..j].to_vec()).unwrap()
    }
}

fn f(s: &str) -> String {
    let b = new_buffer(s);
    b.slice(0, b.arr.len())
}

fn vuln() { // @vuln
    let v = m::src("conv-char-decode-rune-vuln"); // @source(conv-char-decode-rune-vuln) @miss(unmodeled_library_call) @ref(#185)
    m::sink_sql(&f(&v)); // @sink(conv-char-decode-rune-vuln)
}

fn fixed() { // @fixed
    m::sink_sql(&f(&m::clean_sql(&m::src("conv-char-decode-rune-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-decode-rune-decoy"); // @source(conv-char-decode-rune-decoy) @miss(unmodeled_library_call) @ref(#185)
    m::sink_sql(&f("constant")); // @sink(none) @nosink(conv-char-decode-rune-decoy)
    m::sink_cmd(&f(&a)); // @sink(conv-char-decode-rune-decoy)
}
