// @cell(F/pkg/axum/redirect-to)
// @shape: a value of a source goes to `axum::response::Redirect::to` (a sink row of the model: open redirect); the skeleton records it
use axum::response::Redirect;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/axum/redirect-to/vuln", vuln);
    m::case("F/pkg/axum/redirect-to/fixed", fixed);
    m::case("F/pkg/axum/redirect-to/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-axum-redirect-to-vuln"); // @source(pkg-axum-redirect-to-vuln)
    let _ = Redirect::to(&v); // @sink(pkg-axum-redirect-to-vuln)
}

fn fixed() { // @fixed
    let _ = Redirect::to(&m::clean_url(&m::clean_cmd(&m::src("pkg-axum-redirect-to-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-axum-redirect-to-decoy"); // @source(pkg-axum-redirect-to-decoy)
    let _ = Redirect::to("/home"); // @sink(none) @nosink(pkg-axum-redirect-to-decoy)
    let _ = Redirect::to(&t); // @sink(pkg-axum-redirect-to-decoy)
}
