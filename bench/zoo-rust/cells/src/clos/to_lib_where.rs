// @cell(F/clos/to-lib-where)
// @shape: a closure that is given to a function of the standard library that tests the elements of a collection (`Vec::retain`, `Iterator::position`, `Iterator::any`) is called with an element: what the collection holds reaches the sink in the closure. A closure that uses a value that is not an element is not a finding
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-lib-where/vuln", vuln);
    m::case("F/clos/to-lib-where/fixed", fixed);
    m::case("F/clos/to-lib-where/decoy", decoy);
}

fn vuln() { // @vuln
    {
        let t = m::src("clos-to-lib-where-vuln-retain"); // @source(clos-to-lib-where-vuln-retain)
        let mut l = vec![t];
        l.retain(|e| {
            m::sink_cmd(e); // @sink(clos-to-lib-where-vuln-retain)
            true
        });
    }
    {
        let t = m::src("clos-to-lib-where-vuln-position"); // @source(clos-to-lib-where-vuln-position)
        let l = vec![t];
        let _ = l.iter().position(|e| {
            m::sink_cmd(e); // @sink(clos-to-lib-where-vuln-position)
            true
        });
    }
    {
        let t = m::src("clos-to-lib-where-vuln-any"); // @source(clos-to-lib-where-vuln-any)
        let l = vec![t];
        let _ = l.iter().any(|e| {
            m::sink_cmd(e); // @sink(clos-to-lib-where-vuln-any)
            true
        });
    }
}

fn fixed() { // @fixed
    let mut l = vec![m::clean_cmd(&m::src("clos-to-lib-where-fixed"))];
    l.retain(|e| {
        m::sink_cmd(e); // @sink(none)
        true
    });
    let _ = l.iter().position(|e| {
        m::sink_cmd(e); // @sink(none)
        true
    });
    let _ = l.iter().any(|e| {
        m::sink_cmd(e); // @sink(none)
        true
    });
}

fn decoy() { // @decoy
    let t = m::src("clos-to-lib-where-decoy"); // @source(clos-to-lib-where-decoy)
    let mut l = vec!["constant".to_string()];
    l.retain(|e| {
        m::sink_cmd(e); // @sink(none) @nosink(clos-to-lib-where-decoy)
        true
    });
    let _ = l.iter().position(|e| {
        m::sink_cmd(e); // @sink(none) @nosink(clos-to-lib-where-decoy)
        true
    });
    let _ = l.iter().any(|e| {
        m::sink_cmd(e); // @sink(none) @nosink(clos-to-lib-where-decoy)
        true
    });
    m::sink_sql(&t); // @sink(clos-to-lib-where-decoy)
}
