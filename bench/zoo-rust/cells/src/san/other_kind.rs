// @cell(F/san/other-kind)
// @shape: a sanitizer makes the value safe for the sinks of its own kind only (an HTML clean is no clean for a command or a query): the value goes on to the other kinds of sinks
use zoo_mark as m;

pub fn register() {
    m::case("F/san/other-kind/vuln", vuln);
    m::case("F/san/other-kind/vuln2", vuln2);
    m::case("F/san/other-kind/fixed", fixed);
    m::case("F/san/other-kind/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("san-other-kind-vuln"); // @source(san-other-kind-vuln)
    m::sink_cmd(&m::clean_html(&v)); // @sink(san-other-kind-vuln)
}

fn vuln2() { // @vuln
    let v = m::src("san-other-kind-vuln2"); // @source(san-other-kind-vuln2)
    m::sink_html(&m::clean_cmd(&v)); // @sink(san-other-kind-vuln2)
}

fn fixed() { // @fixed
    let v = m::src("san-other-kind-fixed");
    m::sink_html(&m::clean_html(&v)); // @sink(none)
    m::sink_cmd(&m::clean_cmd(&v)); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("san-other-kind-decoy"); // @source(san-other-kind-decoy)
    let h = m::clean_html(&v);
    m::sink_html(&h); // @sink(none) @nosink(san-other-kind-decoy)
    m::sink_sql(&h); // @sink(san-other-kind-decoy)
}
