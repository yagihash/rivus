// @cell(F/pkg/axum/raw-path-params)
// @shape: a handler takes the extractor type RawPathParams of axum (a source row of the model), and its value reaches a sink
use axum::extract::RawPathParams;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/axum/raw-path-params/vuln", vuln);
    m::case("F/pkg/axum/raw-path-params/fixed", fixed);
    m::case("F/pkg/axum/raw-path-params/decoy", decoy);
}

fn handler(x: RawPathParams) { // @source(pkg-axum-raw-path-params-vuln)
    m::sink_cmd(&x.value); // @sink(pkg-axum-raw-path-params-vuln)
}

fn vuln() { // @vuln
    handler(RawPathParams::new("pkg-axum-raw-path-params-vuln"));
}

fn handler_fixed(x: RawPathParams) {
    m::sink_cmd(&m::clean_cmd(&x.value)); // @sink(none)
}

fn fixed() { // @fixed
    handler_fixed(RawPathParams::new("pkg-axum-raw-path-params-fixed"));
}

fn handler_decoy(x: RawPathParams) { // @source(pkg-axum-raw-path-params-decoy)
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-axum-raw-path-params-decoy)
    m::sink_cmd(&x.value); // @sink(pkg-axum-raw-path-params-decoy)
}

fn decoy() { // @decoy
    handler_decoy(RawPathParams::new("pkg-axum-raw-path-params-decoy"));
}
