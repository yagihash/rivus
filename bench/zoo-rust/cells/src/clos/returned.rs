// @cell(F/clos/returned)
// @shape: a function returns a closure that captures its argument, and the caller calls it
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/returned/vuln", vuln);
    m::case("F/clos/returned/fixed", fixed);
    m::case("F/clos/returned/decoy", decoy);
}

fn make1(s: String) -> impl Fn() {
    move || {
        m::sink_cmd(&s); // @sink(clos-returned-vuln) @noisy(4) @ref(ENV-PTR)
    }
}

fn vuln() { // @vuln
    make1(m::src("clos-returned-vuln"))(); // @source(clos-returned-vuln)
}

fn make_fixed(s: String) -> impl Fn() {
    move || {
        m::sink_cmd(&s); // @sink(none) @fp @ref(ENV-PTR)
    }
}

fn fixed() { // @fixed
    make_fixed(m::clean_cmd(&m::src("clos-returned-fixed")))();
}

fn make_decoy(a: String, b: String) -> impl Fn() {
    move || {
        m::sink_cmd(&b); // @sink(none) @fp @nosink(clos-returned-decoy) @ref(ENV-PTR,PARITY)
        m::sink_sql(&a); // @sink(clos-returned-decoy)
    }
}

fn decoy() { // @decoy
    make_decoy(m::src("clos-returned-decoy"), "constant".to_string())(); // @source(clos-returned-decoy)
}
