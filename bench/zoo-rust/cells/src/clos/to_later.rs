// @cell(F/clos/to-later)
// @shape: a closure that captures a value is given to a function of the standard library that calls it later, or in another thread (thread::spawn, thread::scope, Scope::spawn)
use std::thread;
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-later/vuln", vuln);
    m::case("F/clos/to-later/vuln2", vuln2);
    m::case("F/clos/to-later/fixed", fixed);
    m::case("F/clos/to-later/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("clos-to-later-vuln"); // @source(clos-to-later-vuln)
    thread::spawn(move || {
        m::sink_cmd(&v); // @sink(clos-to-later-vuln)
    })
    .join()
    .unwrap();
}

// vuln2: the scope calls the closure in a thread that it joins.
fn vuln2() { // @vuln
    let v = m::src("clos-to-later-vuln2"); // @source(clos-to-later-vuln2)
    thread::scope(|s| {
        s.spawn(|| {
            m::sink_cmd(&v); // @sink(clos-to-later-vuln2)
        });
    });
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("clos-to-later-fixed"));
    thread::spawn(move || {
        m::sink_cmd(&v); // @sink(none)
    })
    .join()
    .unwrap();
}

fn decoy() { // @decoy
    let a = m::src("clos-to-later-decoy"); // @source(clos-to-later-decoy)
    let k = "constant".to_string();
    thread::spawn(move || {
        m::sink_cmd(&k); // @sink(none) @nosink(clos-to-later-decoy)
    })
    .join()
    .unwrap();
    m::sink_sql(&a); // @sink(clos-to-later-decoy)
}
