// @cell(F/conv/char-lexer)
// @shape: a lexer reads the text one char at a time (chars()), collects the chars of a token with String::push and hands out the tokens
// @property: text carried through per-character reads and writes reaches the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/char-lexer/vuln", vuln);
    m::case("F/conv/char-lexer/fixed", fixed);
    m::case("F/conv/char-lexer/decoy", decoy);
}

fn f(s: &str) -> String {
    let mut tokens: Vec<String> = Vec::new();
    let mut cur = String::new();
    for c in s.chars() {
        if c == ' ' {
            tokens.push(cur);
            cur = String::new();
            continue;
        }
        cur.push(c);
    }
    tokens.push(cur);
    tokens.join(" ")
}

fn vuln() { // @vuln
    let v = m::src("conv-char-lexer-vuln"); // @source(conv-char-lexer-vuln) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f(&v)); // @sink(conv-char-lexer-vuln)
}

fn fixed() { // @fixed
    m::sink_cmd(&f(&m::clean_cmd(&m::src("conv-char-lexer-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-char-lexer-decoy"); // @source(conv-char-lexer-decoy) @miss(unmodeled_library_call) @ref(#185)
    m::sink_cmd(&f("a b")); // @sink(none) @nosink(conv-char-lexer-decoy)
    m::sink_sql(&f(&a)); // @sink(conv-char-lexer-decoy)
}
