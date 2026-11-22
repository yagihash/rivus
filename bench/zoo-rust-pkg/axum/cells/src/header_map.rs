// @cell(F/pkg/axum/header-map)
// @shape: a handler takes the extractor type HeaderMap of axum (a source row of the model), and its value reaches a sink
use axum::http::HeaderMap;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/axum/header-map/vuln", vuln);
    m::case("F/pkg/axum/header-map/fixed", fixed);
    m::case("F/pkg/axum/header-map/decoy", decoy);
}

fn handler(x: HeaderMap) { // @source(pkg-axum-header-map-vuln)
    m::sink_cmd(&x.value); // @sink(pkg-axum-header-map-vuln)
}

fn vuln() { // @vuln
    handler(HeaderMap::new("pkg-axum-header-map-vuln"));
}

fn handler_fixed(x: HeaderMap) {
    m::sink_cmd(&m::clean_cmd(&x.value)); // @sink(none)
}

fn fixed() { // @fixed
    handler_fixed(HeaderMap::new("pkg-axum-header-map-fixed"));
}

fn handler_decoy(x: HeaderMap) { // @source(pkg-axum-header-map-decoy)
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-axum-header-map-decoy)
    m::sink_cmd(&x.value); // @sink(pkg-axum-header-map-decoy)
}

fn decoy() { // @decoy
    handler_decoy(HeaderMap::new("pkg-axum-header-map-decoy"));
}
