// @cell(F/clos/to-own-helper)
// @shape: a closure that captures a value is given to a helper of the project that calls it; two callers with their own closures in the decoy
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-own-helper/vuln", vuln);
    m::case("F/clos/to-own-helper/fixed", fixed);
    m::case("F/clos/to-own-helper/decoy", decoy);
}

fn run(f: impl FnOnce()) {
    f()
}

fn vuln() { // @vuln
    let v = m::src("clos-to-own-helper-vuln"); // @source(clos-to-own-helper-vuln)
    run(|| {
        m::sink_cmd(&v); // @sink(clos-to-own-helper-vuln)
    });
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("clos-to-own-helper-fixed"));
    run(|| {
        m::sink_cmd(&v); // @sink(none)
    });
}

fn decoy() { // @decoy
    let v = m::src("clos-to-own-helper-decoy"); // @source(clos-to-own-helper-decoy)
    let k = "constant";
    run(|| {
        m::sink_cmd(k); // @sink(none) @nosink(clos-to-own-helper-decoy)
    });
    run(|| {
        m::sink_sql(&v); // @sink(clos-to-own-helper-decoy)
    });
}
