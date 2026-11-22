// @cell(F/pkg/octocrab/update-file)
// @shape: a value of a source is the path of a file that create_file writes, or the branch it is written to (resource_name)
use octocrab::Octocrab;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/octocrab/update-file/vuln", vuln);
    m::case("F/pkg/octocrab/update-file/fixed", fixed);
    m::case("F/pkg/octocrab/update-file/decoy", decoy);
}

fn vuln() { // @vuln
    let o = Octocrab::default();
    let h = o.repos("owner", "repo");
    let v = m::src("pkg-octocrab-update-file-vuln"); // @source(pkg-octocrab-update-file-vuln)
    let _b = h.create_file(v, "message", "text"); // @sink(pkg-octocrab-update-file-vuln)
}

fn fixed() { // @fixed
    let o = Octocrab::default();
    let h = o.repos("owner", "repo");
    let v = m::clean_cmd(&m::src("pkg-octocrab-update-file-fixed"));
    let _b = h.create_file("docs/a.md", "message", "text").branch(v); // @sink(none)
}

fn decoy() { // @decoy
    let o = Octocrab::default();
    let h = o.repos("owner", "repo");
    let t = m::src("pkg-octocrab-update-file-decoy"); // @source(pkg-octocrab-update-file-decoy)
    let _a = h.create_file("docs/a.md", "message", "text").branch("main"); // @sink(none) @nosink(pkg-octocrab-update-file-decoy)
    let _b = h.create_file("docs/a.md", "message", "text").branch(t); // @sink(pkg-octocrab-update-file-decoy)
}
