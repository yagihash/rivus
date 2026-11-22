// @cell(F/pkg/axum/request-parts)
// @shape: a handler takes the extractor type Parts of axum (a source row of the model), and its value reaches a sink
use axum::http::request::Parts;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/axum/request-parts/vuln", vuln);
    m::case("F/pkg/axum/request-parts/fixed", fixed);
    m::case("F/pkg/axum/request-parts/decoy", decoy);
}

fn handler(x: Parts) { // @source(pkg-axum-request-parts-vuln)
    m::sink_cmd(&x.value); // @sink(pkg-axum-request-parts-vuln)
}

fn vuln() { // @vuln
    handler(Parts::new("pkg-axum-request-parts-vuln"));
}

fn handler_fixed(x: Parts) {
    m::sink_cmd(&m::clean_cmd(&x.value)); // @sink(none)
}

fn fixed() { // @fixed
    handler_fixed(Parts::new("pkg-axum-request-parts-fixed"));
}

fn handler_decoy(x: Parts) { // @source(pkg-axum-request-parts-decoy)
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-axum-request-parts-decoy)
    m::sink_cmd(&x.value); // @sink(pkg-axum-request-parts-decoy)
}

fn decoy() { // @decoy
    handler_decoy(Parts::new("pkg-axum-request-parts-decoy"));
}
