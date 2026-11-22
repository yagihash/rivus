// @cell(F/dyn/sink-as-value-field)
// @shape: a function that a rule names as a sink is made a function value; the function value is stored in a field of a struct and called through the field
// (the field is generic: the function item keeps its type, as a function pointer of a #[track_caller] function loses the line of the call)
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/sink-as-value-field/vuln", vuln);
    m::case("F/dyn/sink-as-value-field/fixed", fixed);
    m::case("F/dyn/sink-as-value-field/decoy", decoy);
}

struct Holder<F: Fn(&str), G: Fn(&str)> {
    cmd: F,
    sql: G,
}

fn vuln() { // @vuln
    let v = m::src("dyn-sink-as-value-field-vuln"); // @source(dyn-sink-as-value-field-vuln)
    let h = Holder { cmd: m::sink_cmd, sql: m::sink_sql };
    (h.cmd)(&v); // @sink(dyn-sink-as-value-field-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("dyn-sink-as-value-field-fixed"));
    let h = Holder { cmd: m::sink_cmd, sql: m::sink_sql };
    (h.cmd)(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dyn-sink-as-value-field-decoy"); // @source(dyn-sink-as-value-field-decoy)
    let h = Holder { cmd: m::sink_cmd, sql: m::sink_sql };
    (h.cmd)("constant"); // @sink(none) @nosink(dyn-sink-as-value-field-decoy)
    (h.sql)(&v); // @sink(dyn-sink-as-value-field-decoy)
}
