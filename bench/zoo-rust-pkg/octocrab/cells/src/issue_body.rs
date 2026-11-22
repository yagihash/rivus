// @cell(F/pkg/octocrab/issue-body)
// @shape: a value that a user of GitHub wrote is read with the field Issue.body of octocrab (a skeleton of it): the source of the built-in rows (a kind: field row)
use octocrab::models::issues::Issue;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/octocrab/issue-body/vuln", vuln);
    m::case("F/pkg/octocrab/issue-body/fixed", fixed);
    m::case("F/pkg/octocrab/issue-body/decoy", decoy);
}

fn issue(tag: &str) -> Issue {
    Issue::zoo(m::In::new(tag).value)
}

fn vuln() { // @vuln
    let i = issue("pkg-octocrab-issue-body-vuln");
    m::sink_cmd(&i.body.unwrap()); // @source(pkg-octocrab-issue-body-vuln) @sink(pkg-octocrab-issue-body-vuln)
}

fn fixed() { // @fixed
    let i = issue("pkg-octocrab-issue-body-fixed");
    m::sink_cmd(&m::clean_cmd(&i.body.unwrap())); // @sink(none)
}

fn decoy() { // @decoy
    let i = issue("pkg-octocrab-issue-body-decoy");
    let v = i.body.unwrap(); // @source(pkg-octocrab-issue-body-decoy)
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-octocrab-issue-body-decoy)
    m::sink_sql(&v); // @sink(pkg-octocrab-issue-body-decoy)
}
