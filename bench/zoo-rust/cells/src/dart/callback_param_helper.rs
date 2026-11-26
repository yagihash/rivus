// @cell(F/dart/callback-param-helper)
// @shape: the route is registered by a helper that is given the function: the function is what the callers of the helper pass, so the parameters of the one that is written at the call of the helper are marked
use zoo_dep::{Handler, Request, Router};
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/callback-param-helper/vuln", vuln);
    m::case("F/dart/callback-param-helper/fixed", fixed);
    m::case("F/dart/callback-param-helper/decoy", decoy);
}

fn typed(tag: &str) -> String {
    m::In::new(tag).value
}

fn mount(r: &mut Router, h: Handler) {
    r.get("/item/{id}", h);
}

fn vuln() { // @vuln
    let mut router = Router::new();
    mount(&mut router, |_req: &Request, id: &str| { // @source(dart-callback-param-helper-vuln)
        m::sink_cmd(id); // @sink(dart-callback-param-helper-vuln)
    });
    let req = Request { path: "/item/x".to_string() };
    router.serve("/item/{id}", &req, &typed("dart-callback-param-helper-vuln"));
}

fn fixed() { // @fixed
    let mut router = Router::new();
    mount(&mut router, |_req: &Request, id: &str| {
        m::sink_cmd(&m::clean_cmd(id)); // @sink(none)
    });
    let req = Request { path: "/item/x".to_string() };
    router.serve("/item/{id}", &req, &typed("dart-callback-param-helper-fixed"));
}

fn decoy() { // @decoy
    let mut router = Router::new();
    mount(&mut router, |req: &Request, id: &str| { // @source(dart-callback-param-helper-decoy)
        m::sink_cmd(&req.path); // @sink(none) @nosink(dart-callback-param-helper-decoy)
        m::sink_sql(id); // @sink(dart-callback-param-helper-decoy)
    });
    let req = Request { path: "/item/x".to_string() };
    router.serve("/item/{id}", &req, &typed("dart-callback-param-helper-decoy"));
}
