// @cell(F/pkg/axum/original-uri)
// @shape: a handler takes the extractor type OriginalUri of axum (a source row of the model), and its value reaches a sink
use axum::extract::OriginalUri;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/axum/original-uri/vuln", vuln);
    m::case("F/pkg/axum/original-uri/fixed", fixed);
    m::case("F/pkg/axum/original-uri/decoy", decoy);
}

fn handler(x: OriginalUri) { // @source(pkg-axum-original-uri-vuln)
    m::sink_cmd(&x.value); // @sink(pkg-axum-original-uri-vuln)
}

fn vuln() { // @vuln
    handler(OriginalUri::new("pkg-axum-original-uri-vuln"));
}

fn handler_fixed(x: OriginalUri) {
    m::sink_cmd(&m::clean_cmd(&x.value)); // @sink(none)
}

fn fixed() { // @fixed
    handler_fixed(OriginalUri::new("pkg-axum-original-uri-fixed"));
}

fn handler_decoy(x: OriginalUri) { // @source(pkg-axum-original-uri-decoy)
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-axum-original-uri-decoy)
    m::sink_cmd(&x.value); // @sink(pkg-axum-original-uri-decoy)
}

fn decoy() { // @decoy
    handler_decoy(OriginalUri::new("pkg-axum-original-uri-decoy"));
}
