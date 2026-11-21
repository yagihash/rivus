// @cell(F/conv/struct-field-boxed)
// @shape: a statement is given to the sink as a `dyn Any` (it is in a box), and the rule of the sink names the field (docs/DESIGN-field-sinks.md)
use std::any::Any;
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/struct-field-boxed/vuln", vuln);
    m::case("F/conv/struct-field-boxed/fixed", fixed);
    m::case("F/conv/struct-field-boxed/decoy", decoy);
}

fn vuln() { // @vuln
    let mut req = m::Stmt::new("");
    req.sql = m::src("conv-struct-field-boxed-vuln"); // @source(conv-struct-field-boxed-vuln) @ref(PARITY)
    let v: Box<dyn Any> = Box::new(req);
    m::sink_stmt_any(v.as_ref()); // @sink(conv-struct-field-boxed-vuln)
}

fn fixed() { // @fixed
    let mut req = m::Stmt::new("");
    req.sql = m::clean_sql(&m::src("conv-struct-field-boxed-fixed"));
    let v: Box<dyn Any> = Box::new(req);
    m::sink_stmt_any(v.as_ref()); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("conv-struct-field-boxed-decoy"); // @source(conv-struct-field-boxed-decoy)
    let mut req = m::Stmt::new("constant");
    req.params.insert("p".to_string(), t.clone());
    let v: Box<dyn Any> = Box::new(req);
    m::sink_stmt_any(v.as_ref()); // @sink(none) @fp @nosink(conv-struct-field-boxed-decoy) @fp @ref(FIELD-SINK,PARITY)
    m::sink_cmd(&t); // @sink(conv-struct-field-boxed-decoy)
}
