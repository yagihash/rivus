// @cell(F/pkg/axum/request)
// @shape: a handler takes the extractor type Request of axum (a source row of the model), and its value reaches a sink
use axum::http::Request;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/axum/request/vuln", vuln);
    m::case("F/pkg/axum/request/fixed", fixed);
    m::case("F/pkg/axum/request/decoy", decoy);
}

fn handler(x: Request) { // @source(pkg-axum-request-vuln)
    m::sink_cmd(&x.value); // @sink(pkg-axum-request-vuln)
}

fn vuln() { // @vuln
    handler(Request::new("pkg-axum-request-vuln"));
}

fn handler_fixed(x: Request) {
    m::sink_cmd(&m::clean_cmd(&x.value)); // @sink(none)
}

fn fixed() { // @fixed
    handler_fixed(Request::new("pkg-axum-request-fixed"));
}

fn handler_decoy(x: Request) { // @source(pkg-axum-request-decoy)
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-axum-request-decoy)
    m::sink_cmd(&x.value); // @sink(pkg-axum-request-decoy)
}

fn decoy() { // @decoy
    handler_decoy(Request::new("pkg-axum-request-decoy"));
}
