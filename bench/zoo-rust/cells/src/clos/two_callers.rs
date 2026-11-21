// @cell(F/clos/two-callers)
// @shape: one helper of the project calls the function it is given with a value; two callers give it their own closures
use zoo_mark as m;

// Each case has its own argument type (a newtype of the marker), so that the cases of the cell do not meet at a call of
// Fn::call, whose candidates are the closures of the same argument types (the same type in one case is the point of the decoy).
struct Tv(String);
struct Tf(String);
struct Td(String);

pub fn register() {
    m::case("F/clos/two-callers/vuln", vuln);
    m::case("F/clos/two-callers/fixed", fixed);
    m::case("F/clos/two-callers/decoy", decoy);
}

fn apply_vuln(f: impl Fn(&Tv), s: &Tv) {
    f(s)
}

fn vuln() { // @vuln
    let v = Tv(m::src("clos-two-callers-vuln")); // @source(clos-two-callers-vuln)
    apply_vuln(
        |x| {
            m::sink_cmd(&x.0); // @sink(clos-two-callers-vuln)
        },
        &v,
    );
}

fn apply_fixed(f: impl Fn(&Tf), s: &Tf) {
    f(s)
}

fn fixed() { // @fixed
    let v = Tf(m::clean_cmd(&m::src("clos-two-callers-fixed")));
    apply_fixed(
        |x| {
            m::sink_cmd(&x.0); // @sink(none)
        },
        &v,
    );
}

fn apply_decoy(f: impl Fn(&Td), s: &Td) {
    f(s)
}

fn decoy_a(s: &Td) {
    apply_decoy(
        |x| {
            m::sink_cmd(&x.0); // @sink(none) @nosink(clos-two-callers-decoy)
        },
        s,
    );
}

fn decoy_b(s: &Td) {
    apply_decoy(
        |x| {
            m::sink_sql(&x.0); // @sink(clos-two-callers-decoy)
        },
        s,
    );
}

fn decoy() { // @decoy
    let v = Td(m::src("clos-two-callers-decoy")); // @source(clos-two-callers-decoy)
    decoy_a(&Td("constant".to_string()));
    decoy_b(&v);
}
