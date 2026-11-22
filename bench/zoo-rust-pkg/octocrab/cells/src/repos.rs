// @cell(F/pkg/octocrab/repos)
// @shape: a value of a source is the owner or the name of the repository that Octocrab::repos reaches (resource_name)
use octocrab::Octocrab;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/octocrab/repos/vuln", vuln);
    m::case("F/pkg/octocrab/repos/fixed", fixed);
    m::case("F/pkg/octocrab/repos/decoy", decoy);
}

fn vuln() { // @vuln
    let o = Octocrab::default();
    let v = m::src("pkg-octocrab-repos-vuln"); // @source(pkg-octocrab-repos-vuln)
    let _h = o.repos("owner", v); // @sink(pkg-octocrab-repos-vuln)
}

fn fixed() { // @fixed
    let o = Octocrab::default();
    let v = m::clean_cmd(&m::src("pkg-octocrab-repos-fixed"));
    let _h = o.repos("owner", v); // @sink(none)
}

fn decoy() { // @decoy
    let o = Octocrab::default();
    let t = m::src("pkg-octocrab-repos-decoy"); // @source(pkg-octocrab-repos-decoy)
    let _a = o.repos("owner", "repo"); // @sink(none) @nosink(pkg-octocrab-repos-decoy)
    let _b = o.repos(t, "repo"); // @sink(pkg-octocrab-repos-decoy)
}
