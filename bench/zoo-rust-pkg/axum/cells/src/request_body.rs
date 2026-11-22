// @cell(F/pkg/axum/request-body)
// @shape: a handler takes the extractor type Body of axum (a source row of the model), and its value reaches a sink
use axum::body::Body;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/axum/request-body/vuln", vuln);
    m::case("F/pkg/axum/request-body/fixed", fixed);
    m::case("F/pkg/axum/request-body/decoy", decoy);
}

fn handler(x: Body) { // @source(pkg-axum-request-body-vuln)
    m::sink_cmd(&x.value); // @sink(pkg-axum-request-body-vuln)
}

fn vuln() { // @vuln
    handler(Body::new("pkg-axum-request-body-vuln"));
}

fn handler_fixed(x: Body) {
    m::sink_cmd(&m::clean_cmd(&x.value)); // @sink(none)
}

fn fixed() { // @fixed
    handler_fixed(Body::new("pkg-axum-request-body-fixed"));
}

fn handler_decoy(x: Body) { // @source(pkg-axum-request-body-decoy)
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-axum-request-body-decoy)
    m::sink_cmd(&x.value); // @sink(pkg-axum-request-body-decoy)
}

fn decoy() { // @decoy
    handler_decoy(Body::new("pkg-axum-request-body-decoy"));
}
