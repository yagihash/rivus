// @cell(F/clos/relay)
// @shape: a function gives the function it was given to a helper that calls it (wrapper(g) calls run(g)); two callers give it their own closures
use zoo_mark as m;

// Each case has its own argument type (a newtype of the marker), so that the cases of the cell do not meet at a call of
// Fn::call, whose candidates are the closures of the same argument types (the same type in one case is the point of the decoy).
struct Tv(String);
struct Tf(String);
struct Td(String);

pub fn register() {
    m::case("F/clos/relay/vuln", vuln);
    m::case("F/clos/relay/fixed", fixed);
    m::case("F/clos/relay/decoy", decoy);
}

fn run_vuln(f: impl Fn(&Tv), s: &Tv) {
    f(s)
}

fn wrapper_vuln(g: impl Fn(&Tv), s: &Tv) {
    run_vuln(g, s)
}

fn vuln() { // @vuln
    let v = Tv(m::src("clos-relay-vuln")); // @source(clos-relay-vuln)
    wrapper_vuln(
        |x| {
            m::sink_cmd(&x.0); // @sink(clos-relay-vuln)
        },
        &v,
    );
}

fn run_fixed(f: impl Fn(&Tf), s: &Tf) {
    f(s)
}

fn wrapper_fixed(g: impl Fn(&Tf), s: &Tf) {
    run_fixed(g, s)
}

fn fixed() { // @fixed
    let v = Tf(m::clean_cmd(&m::src("clos-relay-fixed")));
    wrapper_fixed(
        |x| {
            m::sink_cmd(&x.0); // @sink(none)
        },
        &v,
    );
}

fn run_decoy(f: impl Fn(&Td), s: &Td) {
    f(s)
}

fn wrapper_decoy(g: impl Fn(&Td), s: &Td) {
    run_decoy(g, s)
}

fn decoy_a(s: &Td) {
    wrapper_decoy(
        |x| {
            m::sink_cmd(&x.0); // @sink(none) @fp @nosink(clos-relay-decoy) @fp @ref(XTALK)
        },
        s,
    );
}

fn decoy_b(s: &Td) {
    wrapper_decoy(
        |x| {
            m::sink_sql(&x.0); // @sink(clos-relay-decoy)
        },
        s,
    );
}

fn decoy() { // @decoy
    let v = Td(m::src("clos-relay-decoy")); // @source(clos-relay-decoy)
    decoy_a(&Td("constant".to_string()));
    decoy_b(&v);
}
