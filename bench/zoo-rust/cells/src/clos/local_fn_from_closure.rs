// @cell(F/clos/local-fn-from-closure)
// @shape: a closure made in a function is called from another closure (a callback) in the same function, and the value reaches the sink only through it
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/local-fn-from-closure/vuln", vuln);
    m::case("F/clos/local-fn-from-closure/fixed", fixed);
    m::case("F/clos/local-fn-from-closure/decoy", decoy);
}

fn each(f: impl FnOnce()) {
    f()
}

fn vuln() { // @vuln
    let emit = |s: &str| {
        m::sink_cmd(s); // @sink(clos-local-fn-from-closure-vuln)
    };
    let v = m::src("clos-local-fn-from-closure-vuln"); // @source(clos-local-fn-from-closure-vuln)
    each(|| {
        emit(&v);
    });
}

fn fixed() { // @fixed
    let emit = |s: &str| {
        m::sink_cmd(s); // @sink(none)
    };
    let v = m::clean_cmd(&m::src("clos-local-fn-from-closure-fixed"));
    each(|| {
        emit(&v);
    });
}

fn decoy() { // @decoy
    let emit = |a: &str, b: &str| {
        m::sink_cmd(b); // @sink(none) @nosink(clos-local-fn-from-closure-decoy)
        m::sink_sql(a); // @sink(clos-local-fn-from-closure-decoy)
    };
    let v = m::src("clos-local-fn-from-closure-decoy"); // @source(clos-local-fn-from-closure-decoy)
    each(|| {
        emit(&v, "constant");
    });
}
