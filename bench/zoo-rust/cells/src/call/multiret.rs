// @cell(F/call/multiret)
// @shape: a function returns two values (a tuple), and the caller uses one of them
use zoo_mark as m;

pub fn register() {
    m::case("F/call/multiret/vuln", vuln);
    m::case("F/call/multiret/fixed", fixed);
    m::case("F/call/multiret/decoy", decoy);
}

fn pair(a: String, b: String) -> (String, String) {
    (a, b)
}

fn vuln() { // @vuln
    let (x, _) = pair(m::src("call-multiret-vuln"), "constant".to_string()); // @source(call-multiret-vuln)
    m::sink_cmd(&x); // @sink(call-multiret-vuln)
}

fn fixed() { // @fixed
    let (x, _) = pair(m::clean_cmd(&m::src("call-multiret-fixed")), "constant".to_string());
    m::sink_cmd(&x); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("call-multiret-decoy"); // @source(call-multiret-decoy)
    let (x, y) = pair("constant".to_string(), t);
    m::sink_cmd(&x); // @sink(none) @nosink(call-multiret-decoy)
    m::sink_sql(&y); // @sink(call-multiret-decoy)
}
