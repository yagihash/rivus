// @cell(F/clos/inline)
// @shape: a closure is made and called where it is made
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/inline/vuln", vuln);
    m::case("F/clos/inline/fixed", fixed);
    m::case("F/clos/inline/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("clos-inline-vuln"); // @source(clos-inline-vuln)
    (|| {
        m::sink_cmd(&v); // @sink(clos-inline-vuln) @noisy(4) @ref(ENV-PTR)
    })();
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("clos-inline-fixed"));
    (|| {
        m::sink_cmd(&v); // @sink(none) @fp @ref(ENV-PTR)
    })();
}

fn decoy() { // @decoy
    let v = m::src("clos-inline-decoy"); // @source(clos-inline-decoy)
    let k = "constant";
    (|| {
        m::sink_cmd(k); // @sink(none) @fp @nosink(clos-inline-decoy) @ref(ENV-PTR,PARITY)
    })();
    (|| {
        m::sink_sql(&v); // @sink(clos-inline-decoy) @noisy(4) @ref(ENV-PTR)
    })();
}
