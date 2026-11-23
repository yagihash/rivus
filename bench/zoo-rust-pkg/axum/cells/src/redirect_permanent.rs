// @cell(F/pkg/axum/redirect-permanent)
// @shape: a value of a source goes to `axum::response::Redirect::permanent` (a sink row of the model: open redirect); the skeleton records it
use axum::response::Redirect;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/axum/redirect-permanent/vuln", vuln);
    m::case("F/pkg/axum/redirect-permanent/fixed", fixed);
    m::case("F/pkg/axum/redirect-permanent/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-axum-redirect-permanent-vuln"); // @source(pkg-axum-redirect-permanent-vuln)
    let _ = Redirect::permanent(&v); // @sink(pkg-axum-redirect-permanent-vuln)
}

fn fixed() { // @fixed
    let _ = Redirect::permanent(&m::clean_url(&m::clean_cmd(&m::src("pkg-axum-redirect-permanent-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-axum-redirect-permanent-decoy"); // @source(pkg-axum-redirect-permanent-decoy)
    let _ = Redirect::permanent("/home"); // @sink(none) @nosink(pkg-axum-redirect-permanent-decoy)
    let _ = Redirect::permanent(&t); // @sink(pkg-axum-redirect-permanent-decoy)
}
