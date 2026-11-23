// @cell(F/clos/to-nomodel)
// @shape: a closure that captures a value is given to a function of the standard library that has no row of calls, and the result of the call is used
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-nomodel/vuln", vuln);
    m::case("F/clos/to-nomodel/vuln2", vuln2);
    m::case("F/clos/to-nomodel/fixed", fixed);
    m::case("F/clos/to-nomodel/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("clos-to-nomodel-vuln"); // @source(clos-to-nomodel-vuln)
    let out = std::thread::scope(|_| v.clone());
    m::sink_cmd(&out); // @sink(clos-to-nomodel-vuln)
}

// vuln2: no result. What the receiver holds reaches the parameter of the closure that is given (dedup_by_key has no row).
fn vuln2() { // @vuln
    let v = m::src("clos-to-nomodel-vuln2"); // @source(clos-to-nomodel-vuln2)
    let mut l = vec![v.clone(), v.clone()];
    l.dedup_by_key(|x| {
        m::sink_cmd(x); // @sink(clos-to-nomodel-vuln2)
        0
    });
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("clos-to-nomodel-fixed"));
    let out = std::thread::scope(|_| v.clone());
    m::sink_cmd(&out); // @sink(none)
    let mut l = vec![v.clone(), v.clone()];
    l.dedup_by_key(|x| {
        m::sink_cmd(x); // @sink(none)
        0
    });
}

fn decoy() { // @decoy
    let v = m::src("clos-to-nomodel-decoy"); // @source(clos-to-nomodel-decoy)
    let out = std::thread::scope(|_| String::from("constant"));
    m::sink_cmd(&out); // @sink(none) @nosink(clos-to-nomodel-decoy)
    m::sink_sql(&v); // @sink(clos-to-nomodel-decoy)
    let mut l = vec![String::from("constant"), String::from("constant")];
    l.dedup_by_key(|x| {
        m::sink_cmd(x); // @sink(none) @nosink(clos-to-nomodel-decoy)
        0
    });
}
