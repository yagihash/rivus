// @cell(F/cont/struct-whole-to-sink)
// @shape: a struct with a field that holds the value is given whole to a sink (sink_any prints the fields)
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/struct-whole-to-sink/vuln", vuln);
    m::case("F/cont/struct-whole-to-sink/fixed", fixed);
    m::case("F/cont/struct-whole-to-sink/decoy", decoy);
}

#[derive(Debug)]
struct Pair {
    a: String,
    b: String,
}

// clean_all sanitizes for every class: sink_any judges the text by all of them
fn clean_all(s: &str) -> String {
    m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(s))))
}

fn vuln() { // @vuln
    let p = Pair { a: m::src("cont-struct-whole-to-sink-vuln"), b: "constant".to_string() }; // @source(cont-struct-whole-to-sink-vuln)
    m::sink_any(&p); // @sink(cont-struct-whole-to-sink-vuln)
}

fn fixed() { // @fixed
    let p = Pair { a: clean_all(&m::src("cont-struct-whole-to-sink-fixed")), b: "constant".to_string() };
    m::sink_any(&p); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("cont-struct-whole-to-sink-decoy"); // @source(cont-struct-whole-to-sink-decoy)
    let p = Pair { a: "constant".to_string(), b: "constant".to_string() };
    m::sink_any(&p); // @sink(none) @nosink(cont-struct-whole-to-sink-decoy)
    m::sink_cmd(&t); // @sink(cont-struct-whole-to-sink-decoy)
}
