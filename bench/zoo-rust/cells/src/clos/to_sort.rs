// @cell(F/clos/to-sort)
// @shape: a comparison function that is given to a sort of the standard library (sort_by, sort_by_key) is called with the elements: what the vector holds reaches the sink in the function
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-sort/vuln", vuln);
    m::case("F/clos/to-sort/vuln2", vuln2);
    m::case("F/clos/to-sort/fixed", fixed);
    m::case("F/clos/to-sort/decoy", decoy);
}

fn vuln() { // @vuln
    let t = m::src("clos-to-sort-vuln"); // @source(clos-to-sort-vuln)
    let mut s = vec![t.clone(), t];
    s.sort_by(|a, b| {
        m::sink_cmd(a); // @sink(clos-to-sort-vuln)
        a.cmp(b)
    });
}

// vuln2: the key function is given the elements.
fn vuln2() { // @vuln
    let t = m::src("clos-to-sort-vuln2"); // @source(clos-to-sort-vuln2)
    let mut s = vec![t.clone(), t];
    s.sort_by_key(|a| {
        m::sink_cmd(a); // @sink(clos-to-sort-vuln2)
        a.len()
    });
}

fn fixed() { // @fixed
    let t = m::clean_cmd(&m::src("clos-to-sort-fixed"));
    let mut s = vec![t.clone(), t];
    s.sort_by(|a, b| {
        m::sink_cmd(a); // @sink(none)
        a.cmp(b)
    });
    s.sort_by_key(|a| {
        m::sink_cmd(a); // @sink(none)
        a.len()
    });
}

fn decoy() { // @decoy
    let t = m::src("clos-to-sort-decoy"); // @source(clos-to-sort-decoy)
    let mut s = vec!["x".to_string(), "y".to_string()];
    s.sort_by(|a, b| {
        m::sink_cmd(a); // @sink(none) @nosink(clos-to-sort-decoy)
        a.cmp(b)
    });
    s.sort_by_key(|a| {
        m::sink_cmd(a); // @sink(none) @nosink(clos-to-sort-decoy)
        a.len()
    });
    m::sink_sql(&t); // @sink(clos-to-sort-decoy)
}
