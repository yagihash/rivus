// @cell(F/clos/to-lib-ref)
// @shape: a function that the rules declare a sink, named as a value (not written as a closure), is given to a function of the standard library that calls it with what the receiver holds (Iterator::for_each, Iterator::map, Option::map, Result::and_then): the call is the place of the sink
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-lib-ref/vuln", vuln);
    m::case("F/clos/to-lib-ref/fixed", fixed);
    m::case("F/clos/to-lib-ref/decoy", decoy);
}

fn vuln() { // @vuln
    {
        let t = m::src("clos-to-lib-ref-vuln-foreach"); // @source(clos-to-lib-ref-vuln-foreach)
        vec![t].into_iter().for_each(m::sink_cmd_own); // @sink(clos-to-lib-ref-vuln-foreach)
    }
    {
        let t = m::src("clos-to-lib-ref-vuln-map"); // @source(clos-to-lib-ref-vuln-map)
        let _ = vec![t].into_iter().map(m::sink_cmd_own).count(); // @sink(clos-to-lib-ref-vuln-map)
    }
    {
        let t = m::src("clos-to-lib-ref-vuln-optmap"); // @source(clos-to-lib-ref-vuln-optmap)
        let _ = Some(t).map(m::sink_cmd_own); // @sink(clos-to-lib-ref-vuln-optmap)
    }
    {
        let t = m::src("clos-to-lib-ref-vuln-andthen"); // @source(clos-to-lib-ref-vuln-andthen)
        let _ = Ok::<String, String>(t).and_then(m::sink_cmd_res); // @sink(clos-to-lib-ref-vuln-andthen)
    }
    {
        let t = m::src("clos-to-lib-ref-vuln-variable"); // @source(clos-to-lib-ref-vuln-variable)
        let f = m::sink_cmd_own;
        vec![t].into_iter().for_each(f); // @sink(clos-to-lib-ref-vuln-variable)
    }
    {
        // written as a closure: the sink is the call in it, and the call of the library has none of its own
        let t = m::src("clos-to-lib-ref-vuln-closure"); // @source(clos-to-lib-ref-vuln-closure)
        vec![t].into_iter().for_each(|e| {
            m::sink_cmd_own(e); // @sink(clos-to-lib-ref-vuln-closure)
        });
    }
}

fn fixed() { // @fixed
    let t = m::clean_cmd(&m::src("clos-to-lib-ref-fixed"));
    vec![t.clone()].into_iter().for_each(m::sink_cmd_own); // @sink(none)
    let _ = vec![t.clone()].into_iter().map(m::sink_cmd_own).count(); // @sink(none)
    let _ = Some(t.clone()).map(m::sink_cmd_own); // @sink(none)
    let _ = Ok::<String, String>(t.clone()).and_then(m::sink_cmd_res); // @sink(none)
    let f = m::sink_cmd_own;
    vec![t].into_iter().for_each(f); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("clos-to-lib-ref-decoy"); // @source(clos-to-lib-ref-decoy)
    // what the receiver holds is a constant: the value of the source is not what the library hands to the function
    let k = || "constant".to_string();
    vec![k()].into_iter().for_each(m::sink_cmd_own); // @sink(none) @nosink(clos-to-lib-ref-decoy)
    let _ = vec![k()].into_iter().map(m::sink_cmd_own).count(); // @sink(none) @nosink(clos-to-lib-ref-decoy)
    let _ = Some(k()).map(m::sink_cmd_own); // @sink(none) @nosink(clos-to-lib-ref-decoy)
    let _ = Ok::<String, String>(k()).and_then(m::sink_cmd_res); // @sink(none) @nosink(clos-to-lib-ref-decoy)
    let f = m::sink_cmd_own;
    vec![k()].into_iter().for_each(f); // @sink(none) @nosink(clos-to-lib-ref-decoy)
    // a function that is no sink, given the tainted elements
    vec![t.clone()].into_iter().for_each(is_empty);
    m::sink_sql(&t); // @sink(clos-to-lib-ref-decoy)
}

fn is_empty(s: String) {
    let _ = s.is_empty();
}
