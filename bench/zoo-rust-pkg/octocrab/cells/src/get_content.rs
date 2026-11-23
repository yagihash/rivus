// @cell(F/pkg/octocrab/get-content)
// @shape: a value of a source is the path or the ref that get_content of a repository reaches (resource_name)
use octocrab::Octocrab;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/octocrab/get-content/vuln", vuln);
    m::case("F/pkg/octocrab/get-content/fixed", fixed);
    m::case("F/pkg/octocrab/get-content/decoy", decoy);
}

fn vuln() { // @vuln
    let o = Octocrab::default();
    let h = o.repos("owner", "repo");
    let v = m::src("pkg-octocrab-get-content-vuln"); // @source(pkg-octocrab-get-content-vuln)
    let _b = h.get_content().path("docs").r#ref(v); // @sink(pkg-octocrab-get-content-vuln)
}

fn fixed() { // @fixed
    let o = Octocrab::default();
    let h = o.repos("owner", "repo");
    let v = m::clean_resource(&m::clean_cmd(&m::src("pkg-octocrab-get-content-fixed")));
    let _b = h.get_content().path(v); // @sink(none)
}

fn decoy() { // @decoy
    let o = Octocrab::default();
    let h = o.repos("owner", "repo");
    let t = m::src("pkg-octocrab-get-content-decoy"); // @source(pkg-octocrab-get-content-decoy)
    let _a = h.get_content().path("docs").r#ref("main"); // @sink(none) @nosink(pkg-octocrab-get-content-decoy)
    let _b = h.get_content().path(t); // @sink(pkg-octocrab-get-content-decoy)
}
