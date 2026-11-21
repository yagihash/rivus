// @cell(F/guard/one-branch)
// @shape: only one branch sanitizes (the case runs both branches)
use zoo_mark as m;

pub fn register() {
    m::case("F/guard/one-branch/vuln", vuln);
    m::case("F/guard/one-branch/fixed", fixed);
    m::case("F/guard/one-branch/decoy", decoy);
}

fn branch(mut s: String, ok: bool) {
    if ok {
        s = m::clean_cmd(&s);
    }
    m::sink_cmd(&s); // @sink(guard-one-branch-vuln)
}

fn vuln() { // @vuln
    let v = m::src("guard-one-branch-vuln"); // @source(guard-one-branch-vuln)
    branch(v.clone(), true);
    branch(v, false);
}

fn branch_fixed(mut s: String, ok: bool) {
    if ok {
        s = m::clean_cmd(&s);
    } else {
        s = m::clean_cmd(&s);
    }
    m::sink_cmd(&s); // @sink(none) @fp @ref(PARITY)
}

fn fixed() { // @fixed
    let v = m::src("guard-one-branch-fixed");
    branch_fixed(v.clone(), true);
    branch_fixed(v, false);
}

fn branch_decoy(mut a: String, b: &str, ok: bool) {
    if ok {
        a = m::clean_cmd(&a);
    }
    m::sink_cmd(b); // @sink(none) @nosink(guard-one-branch-decoy)
    m::sink_sql(&a); // @sink(guard-one-branch-decoy)
}

fn decoy() { // @decoy
    let v = m::src("guard-one-branch-decoy"); // @source(guard-one-branch-decoy)
    branch_decoy(v.clone(), "constant", true);
    branch_decoy(v, "constant", false);
}
