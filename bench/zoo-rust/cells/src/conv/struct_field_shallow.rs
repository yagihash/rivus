// @cell(F/conv/struct-field-shallow)
// @shape: a statement (the SQL is the dangerous field, the bind parameters are not) is given whole to the call that runs it, and the rule of the sink names the field (docs/DESIGN-field-sinks.md)
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/struct-field-shallow/vuln", vuln);
    m::case("F/conv/struct-field-shallow/fixed", fixed);
    m::case("F/conv/struct-field-shallow/decoy", decoy);
}

fn vuln() { // @vuln
    let mut req = m::Stmt::new("");
    req.sql = m::src("conv-struct-field-shallow-vuln"); // @source(conv-struct-field-shallow-vuln) @ref(PARITY)
    m::sink_stmt(&req); // @sink(conv-struct-field-shallow-vuln)
}

fn fixed() { // @fixed
    let mut req = m::Stmt::new("");
    req.sql = m::clean_sql(&m::src("conv-struct-field-shallow-fixed"));
    m::sink_stmt(&req); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("conv-struct-field-shallow-decoy"); // @source(conv-struct-field-shallow-decoy)
    let mut req = m::Stmt::new("constant");
    req.params.insert("p".to_string(), t.clone());
    m::sink_stmt(&req); // @sink(none) @nosink(conv-struct-field-shallow-decoy)
    m::sink_cmd(&t); // @sink(conv-struct-field-shallow-decoy)
}
