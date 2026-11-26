// @cell(F/clos/wrapped-by-dependency)
// @shape: a closure of the project is given to a dependency (zoo_dep::wrap_fn), which returns a closure that calls it; the project calls what it got back
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/wrapped-by-dependency/vuln", vuln);
    m::case("F/clos/wrapped-by-dependency/fixed", fixed);
    m::case("F/clos/wrapped-by-dependency/decoy", decoy);
}

fn vuln() { // @vuln
    let a = m::src("clos-wrapped-by-dependency-vuln"); // @source(clos-wrapped-by-dependency-vuln) @miss(unmodeled_library_call) @ref(#78,PARITY)
    let g = zoo_dep::wrap_fn(|s| s);
    m::sink_cmd(&g(a)); // @sink(clos-wrapped-by-dependency-vuln)
}

fn fixed() { // @fixed
    let a = m::src("clos-wrapped-by-dependency-fixed");
    let g = zoo_dep::wrap_fn(|s| m::clean_cmd(&s));
    m::sink_cmd(&g(a)); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("clos-wrapped-by-dependency-decoy"); // @source(clos-wrapped-by-dependency-decoy) @miss(unmodeled_library_call) @ref(#78)
    let quiet = zoo_dep::wrap_fn(|_| "constant".to_string());
    let loud = zoo_dep::wrap_fn(|s| s);
    m::sink_cmd(&quiet(a.clone())); // @sink(none) @nosink(clos-wrapped-by-dependency-decoy)
    m::sink_sql(&loud(a)); // @sink(clos-wrapped-by-dependency-decoy)
}
