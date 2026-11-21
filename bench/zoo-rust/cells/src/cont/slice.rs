// @cell(F/cont/slice)
// @shape: a value is an element of a Vec literal, and the element is read by its index (the decoy reads the other element)
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/slice/vuln", vuln);
    m::case("F/cont/slice/fixed", fixed);
    m::case("F/cont/slice/decoy", decoy);
}

fn vuln() { // @vuln
    let s = vec![m::src("cont-slice-vuln"), "constant".to_string()]; // @source(cont-slice-vuln)
    m::sink_cmd(&s[0]); // @sink(cont-slice-vuln)
}

fn fixed() { // @fixed
    let s = vec![m::clean_cmd(&m::src("cont-slice-fixed")), "constant".to_string()];
    m::sink_cmd(&s[0]); // @sink(none)
}

fn decoy() { // @decoy
    let s = vec![m::src("cont-slice-decoy"), "constant".to_string()]; // @source(cont-slice-decoy)
    m::sink_cmd(&s[1]); // @sink(none) @fp @nosink(cont-slice-decoy) @fp @ref(KEYED-MAP,PARITY)
    m::sink_sql(&s[0]); // @sink(cont-slice-decoy)
}
