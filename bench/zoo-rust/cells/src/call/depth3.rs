// @cell(F/call/depth3)
// @shape: a value goes through three calls (each passes it on) to a sink
use zoo_mark as m;

pub fn register() {
    m::case("F/call/depth3/vuln", vuln);
    m::case("F/call/depth3/fixed", fixed);
    m::case("F/call/depth3/decoy", decoy);
}

fn f1(s: &str) {
    f2(s)
}
fn f2(s: &str) {
    f3(s)
}
fn f3(s: &str) {
    m::sink_cmd(s); // @sink(call-depth3-vuln)
}

fn vuln() { // @vuln
    f1(&m::src("call-depth3-vuln")); // @source(call-depth3-vuln)
}

fn g1(s: &str) {
    g2(s)
}
fn g2(s: &str) {
    g3(s)
}
fn g3(s: &str) {
    m::sink_cmd(s); // @sink(none)
}

fn fixed() { // @fixed
    g1(&m::clean_cmd(&m::src("call-depth3-fixed")));
}

// h1 passes both of its arguments down; only the first one is tainted, and it reaches the SQL sink
fn h1(a: &str, b: &str) {
    h2(a, b)
}
fn h2(a: &str, b: &str) {
    h3(a, b)
}
fn h3(a: &str, b: &str) {
    m::sink_cmd(b); // @sink(none) @nosink(call-depth3-decoy)
    m::sink_sql(a); // @sink(call-depth3-decoy)
}

fn decoy() { // @decoy
    let t = m::src("call-depth3-decoy"); // @source(call-depth3-decoy)
    h1(&t, "constant");
}
