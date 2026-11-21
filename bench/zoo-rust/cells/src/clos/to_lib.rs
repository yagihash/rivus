// @cell(F/clos/to-lib)
// @shape: a closure that captures a value is given to a function of the standard library that calls it (Once::call_once, OnceLock::get_or_init)
use std::sync::{Once, OnceLock};
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-lib/vuln", vuln);
    m::case("F/clos/to-lib/vuln2", vuln2);
    m::case("F/clos/to-lib/fixed", fixed);
    m::case("F/clos/to-lib/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("clos-to-lib-vuln"); // @source(clos-to-lib-vuln) @miss(unmodeled_library_call) @ref(CALLBACK-MODEL,PARITY)
    let once = Once::new();
    once.call_once(|| {
        m::sink_cmd(&v); // @sink(clos-to-lib-vuln)
    });
}

// vuln2: the result of the closure comes back through the library.
fn vuln2() { // @vuln
    let v = m::src("clos-to-lib-vuln2"); // @source(clos-to-lib-vuln2)
    let cell: OnceLock<String> = OnceLock::new();
    let r = cell.get_or_init(|| v.clone());
    m::sink_cmd(r); // @sink(clos-to-lib-vuln2)
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("clos-to-lib-fixed"));
    let once = Once::new();
    once.call_once(|| {
        m::sink_cmd(&v); // @sink(none)
    });
}

fn decoy() { // @decoy
    let v = m::src("clos-to-lib-decoy"); // @source(clos-to-lib-decoy)
    let k = "constant";
    let once = Once::new();
    once.call_once(|| {
        m::sink_cmd(k); // @sink(none) @nosink(clos-to-lib-decoy)
    });
    let cell: OnceLock<String> = OnceLock::new();
    let r = cell.get_or_init(|| v.clone());
    m::sink_sql(r); // @sink(clos-to-lib-decoy)
}
