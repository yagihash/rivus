// @cell(F/rust/ownership-move)
// @shape: a String is moved (let t = s; a call that takes it by value; a for loop over a Vec that owns it) and the sink takes the owner
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/ownership-move/vuln", vuln);
    m::case("F/rust/ownership-move/fixed", fixed);
    m::case("F/rust/ownership-move/decoy", decoy);
    m::case("F/rust/ownership-move/vuln2", vuln2);
}

fn consume(s: String) {
    m::sink_cmd_own(s); // @sink(rust-ownership-move-vuln)
}

fn vuln() { // @vuln
    let s = m::src("rust-ownership-move-vuln"); // @source(rust-ownership-move-vuln)
    let t = s;
    consume(t);
}

fn vuln2() { // @vuln
    let v = vec![m::src("rust-ownership-move-vuln2")]; // @source(rust-ownership-move-vuln2)
    for x in v {
        m::sink_cmd_own(x); // @sink(rust-ownership-move-vuln2)
    }
}

fn consume_fixed(s: String) {
    m::sink_cmd_own(s); // @sink(none)
}

fn fixed() { // @fixed
    let s = m::clean_cmd(&m::src("rust-ownership-move-fixed"));
    let t = s;
    consume_fixed(t);
}

fn consume_decoy(a: String, b: String) {
    m::sink_cmd_own(b); // @sink(none) @nosink(rust-ownership-move-decoy)
    m::sink_sql(&a); // @sink(rust-ownership-move-decoy)
}

fn decoy() { // @decoy
    let a = m::src("rust-ownership-move-decoy"); // @source(rust-ownership-move-decoy)
    consume_decoy(a, "constant".to_string());
}
