// @cell(F/conv/struct-field-opaque)
// @shape: a sibling of the dangerous field is tainted, and the structure goes through a function that the analysis has no body and no model of (a function of a dependency), which moves it into the dangerous field (docs/DESIGN-field-sinks.md)
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/struct-field-opaque/vuln", vuln);
    m::case("F/conv/struct-field-opaque/fixed", fixed);
    m::case("F/conv/struct-field-opaque/decoy", decoy);
}

fn vuln() { // @vuln
    let mut req = m::Stmt::new("");
    req.params.insert("body".to_string(), m::src("conv-struct-field-opaque-vuln")); // @source(conv-struct-field-opaque-vuln) @ref(PARITY)
    zoo_dep::opaque(&mut req);
    m::sink_stmt(&req); // @sink(conv-struct-field-opaque-vuln)
}

fn fixed() { // @fixed
    let mut req = m::Stmt::new("");
    req.params.insert("body".to_string(), m::clean_sql(&m::src("conv-struct-field-opaque-fixed")));
    zoo_dep::opaque(&mut req);
    m::sink_stmt(&req); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("conv-struct-field-opaque-decoy"); // @source(conv-struct-field-opaque-decoy)
    let mut req = m::Stmt::new("constant");
    req.params.insert("body".to_string(), t.clone());
    let mut other = m::Stmt::new("");
    zoo_dep::opaque(&mut other);
    m::sink_stmt(&req); // @sink(none) @nosink(conv-struct-field-opaque-decoy)
    m::sink_cmd(&t); // @sink(conv-struct-field-opaque-decoy)
}
