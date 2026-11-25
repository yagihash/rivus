// @cell(F/clos/to-nomodel-ref)
// @shape: a function that the rules declare a sink, named as a value (not written as a closure), is given to a function of the standard library that has no body and no row of calls (Vec::dedup_by_key): the call is the place of the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-nomodel-ref/vuln", vuln);
    m::case("F/clos/to-nomodel-ref/fixed", fixed);
    m::case("F/clos/to-nomodel-ref/decoy", decoy);
}

fn vuln() { // @vuln
    {
        let t = m::src("clos-to-nomodel-ref-vuln-direct"); // @source(clos-to-nomodel-ref-vuln-direct)
        let mut l = vec![t.clone(), t];
        l.dedup_by_key(m::sink_cmd_key); // @sink(clos-to-nomodel-ref-vuln-direct)
    }
    {
        let t = m::src("clos-to-nomodel-ref-vuln-variable"); // @source(clos-to-nomodel-ref-vuln-variable)
        let mut l = vec![t.clone(), t];
        let f = m::sink_cmd_key;
        l.dedup_by_key(f); // @sink(clos-to-nomodel-ref-vuln-variable)
    }
    {
        // written as a closure: the sink is the call in it, and the call of the library has none of its own
        let t = m::src("clos-to-nomodel-ref-vuln-closure"); // @source(clos-to-nomodel-ref-vuln-closure)
        let mut l = vec![t.clone(), t];
        l.dedup_by_key(|x| {
            m::sink_cmd_key(x) // @sink(clos-to-nomodel-ref-vuln-closure)
        });
    }
}

fn fixed() { // @fixed
    let t = m::clean_cmd(&m::src("clos-to-nomodel-ref-fixed"));
    let mut l = vec![t.clone(), t.clone()];
    l.dedup_by_key(m::sink_cmd_key); // @sink(none)
    let mut l2 = vec![t.clone(), t];
    let f = m::sink_cmd_key;
    l2.dedup_by_key(f); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("clos-to-nomodel-ref-decoy"); // @source(clos-to-nomodel-ref-decoy)
    // the elements are constants: the value of the source is not what the library hands to the function
    let k = || "constant".to_string();
    let mut l = vec![k(), k()];
    l.dedup_by_key(m::sink_cmd_key); // @sink(none) @nosink(clos-to-nomodel-ref-decoy)
    let mut l2 = vec![k(), k()];
    let f = m::sink_cmd_key;
    l2.dedup_by_key(f); // @sink(none) @nosink(clos-to-nomodel-ref-decoy)
    // a function that is no sink, given the tainted elements
    let mut q = vec![t.clone(), t.clone()];
    q.dedup_by_key(ignore);
    m::sink_sql(&t); // @sink(clos-to-nomodel-ref-decoy)
}

fn ignore(_s: &mut String) -> i32 {
    0
}
