// @cell(F/clos/to-lib-call)
// @shape: a closure that captures a value is given to a method of the standard library that calls it with what the receiver holds (Iterator::for_each, Result::and_then, Option::map_or_else), and the closure runs in that call
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-lib-call/vuln", vuln);
    m::case("F/clos/to-lib-call/vuln2", vuln2);
    m::case("F/clos/to-lib-call/fixed", fixed);
    m::case("F/clos/to-lib-call/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("clos-to-lib-call-vuln"); // @source(clos-to-lib-call-vuln)
    [1].iter().for_each(|_| {
        m::sink_cmd(&v); // @sink(clos-to-lib-call-vuln)
    });
}

// vuln2: the closure is given what the Result holds.
fn vuln2() { // @vuln
    let v = m::src("clos-to-lib-call-vuln2"); // @source(clos-to-lib-call-vuln2)
    let r: Result<String, String> = Ok(v);
    let _ = r.and_then(|s| {
        m::sink_cmd(&s); // @sink(clos-to-lib-call-vuln2)
        Ok::<String, String>(s)
    });
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("clos-to-lib-call-fixed"));
    [1].iter().for_each(|_| {
        m::sink_cmd(&v); // @sink(none)
    });
    let r: Result<String, String> = Ok(v);
    let _ = r.and_then(|s| {
        m::sink_cmd(&s); // @sink(none)
        Ok::<String, String>(s)
    });
}

fn decoy() { // @decoy
    let v = m::src("clos-to-lib-call-decoy"); // @source(clos-to-lib-call-decoy)
    let k = "constant".to_string();
    [1].iter().for_each(|_| {
        m::sink_cmd(&k); // @sink(none) @nosink(clos-to-lib-call-decoy)
    });
    let r: Result<String, String> = Ok(k);
    let _ = r.and_then(|s| {
        m::sink_cmd(&s); // @sink(none) @nosink(clos-to-lib-call-decoy)
        Ok::<String, String>(s)
    });
    m::sink_sql(&v); // @sink(clos-to-lib-call-decoy)
}
