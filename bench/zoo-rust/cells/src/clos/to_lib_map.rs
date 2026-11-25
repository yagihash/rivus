// @cell(F/clos/to-lib-map)
// @shape: a closure given to `HashMap::retain` is called with the key and the value: what the map holds reaches the sink in the closure. A closure that uses a value that is not an entry is not a finding
use std::collections::HashMap;
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/to-lib-map/vuln", vuln);
    m::case("F/clos/to-lib-map/fixed", fixed);
    m::case("F/clos/to-lib-map/decoy", decoy);
}

fn vuln() { // @vuln
    {
        let t = m::src("clos-to-lib-map-vuln-key"); // @source(clos-to-lib-map-vuln-key)
        let mut l: HashMap<String, String> = HashMap::new();
        l.insert(t, "v".to_string());
        l.retain(|k, _v| {
            m::sink_cmd(k); // @sink(clos-to-lib-map-vuln-key)
            true
        });
    }
    {
        let t = m::src("clos-to-lib-map-vuln-value"); // @source(clos-to-lib-map-vuln-value)
        let mut l: HashMap<String, String> = HashMap::new();
        l.insert("k".to_string(), t);
        l.retain(|_k, v| {
            m::sink_cmd(v); // @sink(clos-to-lib-map-vuln-value)
            true
        });
    }
}

fn fixed() { // @fixed
    let mut l: HashMap<String, String> = HashMap::new();
    l.insert("k".to_string(), m::clean_cmd(&m::src("clos-to-lib-map-fixed")));
    l.retain(|_k, v| {
        m::sink_cmd(v); // @sink(none)
        true
    });
}

fn decoy() { // @decoy
    let t = m::src("clos-to-lib-map-decoy"); // @source(clos-to-lib-map-decoy)
    let mut l: HashMap<String, String> = HashMap::new();
    l.insert("k".to_string(), "constant".to_string());
    l.retain(|_k, v| {
        m::sink_cmd(v); // @sink(none) @nosink(clos-to-lib-map-decoy)
        true
    });
    m::sink_sql(&t); // @sink(clos-to-lib-map-decoy)
}
