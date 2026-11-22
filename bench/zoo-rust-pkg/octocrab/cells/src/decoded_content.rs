// @cell(F/pkg/octocrab/decoded-content)
// @shape: the text of a file of a repository, which a user of GitHub wrote, is read with Content::decoded_content: the source of the built-in rows
use octocrab::models::repos::Content;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/octocrab/decoded-content/vuln", vuln);
    m::case("F/pkg/octocrab/decoded-content/fixed", fixed);
    m::case("F/pkg/octocrab/decoded-content/decoy", decoy);
}

// (the value is made with In::new, which is not a source of the rules: the source of the cell is the call of the package)
fn file(tag: &str) -> Content {
    Content::zoo(m::In::new(tag).value)
}

fn vuln() { // @vuln
    let c = file("pkg-octocrab-decoded-content-vuln");
    m::sink_cmd(&c.decoded_content().unwrap()); // @source(pkg-octocrab-decoded-content-vuln) @sink(pkg-octocrab-decoded-content-vuln)
}

fn fixed() { // @fixed
    let c = file("pkg-octocrab-decoded-content-fixed");
    m::sink_cmd(&m::clean_cmd(&c.decoded_content().unwrap())); // @sink(none)
}

fn decoy() { // @decoy
    let c = file("pkg-octocrab-decoded-content-decoy");
    let v = c.decoded_content().unwrap(); // @source(pkg-octocrab-decoded-content-decoy)
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-octocrab-decoded-content-decoy)
    m::sink_sql(&v); // @sink(pkg-octocrab-decoded-content-decoy)
}
