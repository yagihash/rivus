// @cell(F/clos/src-ref)
// @shape: a function that the rules declare a source, named as a value (not written as a closure), is given to a function of the standard library that calls it and puts what it returns in its result (Option::map, Iterator::map): the call is the place of the source
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/src-ref/vuln", vuln);
    m::case("F/clos/src-ref/fixed", fixed);
    m::case("F/clos/src-ref/decoy", decoy);
}

fn vuln() { // @vuln
    {
        let t = Some("clos-src-ref-vuln-optmap").map(m::src).unwrap(); // @source(clos-src-ref-vuln-optmap)
        m::sink_cmd(&t); // @sink(clos-src-ref-vuln-optmap)
    }
    {
        let v: Vec<String> = vec!["clos-src-ref-vuln-map"].into_iter().map(m::src).collect(); // @source(clos-src-ref-vuln-map)
        m::sink_cmd(&v[0]); // @sink(clos-src-ref-vuln-map)
    }
    {
        let f = m::src;
        let t = Some("clos-src-ref-vuln-variable").map(f).unwrap(); // @source(clos-src-ref-vuln-variable)
        m::sink_cmd(&t); // @sink(clos-src-ref-vuln-variable)
    }
    {
        // written as a closure: the source is the call in it, and the call of the library has none of its own
        let t = Some("x")
            .map(|_| m::src("clos-src-ref-vuln-closure")) // @source(clos-src-ref-vuln-closure)
            .unwrap();
        m::sink_cmd(&t); // @sink(clos-src-ref-vuln-closure)
    }
}

fn fixed() { // @fixed
    let t = m::clean_cmd(&Some("clos-src-ref-fixed-optmap").map(m::src).unwrap());
    m::sink_cmd(&t); // @sink(none)
    let v: Vec<String> = vec!["clos-src-ref-fixed-map"].into_iter().map(m::src).collect();
    let u = m::clean_cmd(&v[0]);
    m::sink_cmd(&u); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("clos-src-ref-decoy"); // @source(clos-src-ref-decoy)
    // a function that is no source: what it returns is not tainted
    let a = Some("constant").map(plain).unwrap();
    m::sink_cmd(&a); // @sink(none) @nosink(clos-src-ref-decoy)
    // a source that the call puts nowhere (the result of the call is a bool)
    let b = vec!["constant".to_string()].into_iter().any(m::src_b);
    m::sink_cmd(&b.to_string()); // @sink(none) @nosink(clos-src-ref-decoy)
    m::sink_sql(&t); // @sink(clos-src-ref-decoy)
}

fn plain(s: &str) -> String {
    format!("{s}!")
}
