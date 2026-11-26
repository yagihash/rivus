// @cell(F/dart/callback-param-named)
// @shape: a function that has a name (and a variable that holds a function written once) is given to the router in place of a closure written there: it is found as well, and its parameter after the request is user input; the other two arguments of the same name are not
use zoo_dep::{Request, Router};
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/callback-param-named/vuln", vuln);
    m::case("F/dart/callback-param-named/fixed", fixed);
    m::case("F/dart/callback-param-named/decoy", decoy);
}

fn typed(tag: &str) -> String {
    m::In::new(tag).value
}

fn show_item(_req: &Request, id: &str) { // @source(dart-callback-param-named-vuln)
    m::sink_cmd(id); // @sink(dart-callback-param-named-vuln)
}

fn vuln() { // @vuln
    let mut router = Router::new();
    router.get("/item/{id}", show_item);
    let req = Request { path: "/item/x".to_string() };
    router.serve("/item/{id}", &req, &typed("dart-callback-param-named-vuln"));
}

fn show_item_fixed(_req: &Request, id: &str) {
    m::sink_cmd(&m::clean_cmd(id)); // @sink(none)
}

fn fixed() { // @fixed
    let mut router = Router::new();
    router.get("/item/{id}", show_item_fixed);
    let req = Request { path: "/item/x".to_string() };
    router.serve("/item/{id}", &req, &typed("dart-callback-param-named-fixed"));
}

fn decoy() { // @decoy
    let mut router = Router::new();
    let handler = |req: &Request, id: &str| { // @source(dart-callback-param-named-decoy)
        m::sink_cmd(&req.path); // @sink(none) @nosink(dart-callback-param-named-decoy)
        m::sink_sql(id); // @sink(dart-callback-param-named-decoy)
    };
    router.get("/item/{id}", handler);
    let req = Request { path: "/item/x".to_string() };
    router.serve("/item/{id}", &req, &typed("dart-callback-param-named-decoy"));
}
