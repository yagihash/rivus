// @cell(F/guard/reapply-loop)
// @shape: a call is applied again in a loop, and the return after the loop depends on a parameter (the case runs both values)
use zoo_mark as m;

pub fn register() {
    m::case("F/guard/reapply-loop/vuln", vuln);
    m::case("F/guard/reapply-loop/fixed", fixed);
    m::case("F/guard/reapply-loop/decoy", decoy);
}

fn pass(flag: bool, s: String) -> String {
    if flag {
        return s;
    }
    String::new()
}

// the call is applied again with what it gave back (r + s), and skip decides what is returned
fn looped(skip: bool, s: &str) -> String {
    let mut r = String::new();
    for _ in 0..3 {
        r = pass(true, format!("{}{}", r, s));
    }
    if skip {
        return String::new();
    }
    r
}

fn via_sink(skip: bool, s: &str) {
    m::sink_cmd(&looped(skip, s)); // @sink(guard-reapply-loop-vuln)
}

fn vuln() { // @vuln
    let v = m::src("guard-reapply-loop-vuln"); // @source(guard-reapply-loop-vuln)
    via_sink(false, &v);
    via_sink(true, &v);
    let _ = pass(false, v.clone()); // the way of pass that gives back "" (the loop calls it with true)
}

fn via_sink_fixed(skip: bool, s: &str) {
    m::sink_cmd(&m::clean_cmd(&looped(skip, s))); // @sink(none)
}

fn fixed() { // @fixed
    let v = m::src("guard-reapply-loop-fixed");
    via_sink_fixed(false, &v);
    via_sink_fixed(true, &v);
}

// skip is true on the line of the command: the function gives back "" there, and the value reaches the other line only
fn decoy() { // @decoy
    let v = m::src("guard-reapply-loop-decoy"); // @source(guard-reapply-loop-decoy)
    m::sink_cmd(&looped(true, &v)); // @sink(none) @fp @nosink(guard-reapply-loop-decoy) @fp @ref(PARITY)
    m::sink_sql(&looped(false, &v)); // @sink(guard-reapply-loop-decoy)
}
