// @cell(F/pkg/axum/redirect-temporary)
// @shape: a value of a source goes to `axum::response::Redirect::temporary` (a sink row of the model: open redirect); the skeleton records it
use axum::response::Redirect;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/axum/redirect-temporary/vuln", vuln);
    m::case("F/pkg/axum/redirect-temporary/fixed", fixed);
    m::case("F/pkg/axum/redirect-temporary/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-axum-redirect-temporary-vuln"); // @source(pkg-axum-redirect-temporary-vuln)
    let _ = Redirect::temporary(&v); // @sink(pkg-axum-redirect-temporary-vuln)
}

fn fixed() { // @fixed
    let _ = Redirect::temporary(&m::clean_url(&m::clean_cmd(&m::src("pkg-axum-redirect-temporary-fixed")))); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-axum-redirect-temporary-decoy"); // @source(pkg-axum-redirect-temporary-decoy)
    let _ = Redirect::temporary("/home"); // @sink(none) @nosink(pkg-axum-redirect-temporary-decoy)
    let _ = Redirect::temporary(&t); // @sink(pkg-axum-redirect-temporary-decoy)
}
