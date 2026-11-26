// @cell(F/rust/macro-closure-twice)
// @shape: a macro_rules! macro that makes a closure, called twice, so that the two closures come from the one place in the macro (the type of their environments is the same); vuln, fixed, decoy: the closure captures the argument and is passed to a helper that calls it; vuln2: the closure is a handler that the macro registers to a router (callback_param of the rules)
use zoo_dep::{Request, Router};
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/macro-closure-twice/vuln", vuln);
    m::case("F/rust/macro-closure-twice/fixed", fixed);
    m::case("F/rust/macro-closure-twice/decoy", decoy);
    m::case("F/rust/macro-closure-twice/vuln2", vuln2);
}

// The closures of this cell take two arguments, and no other closure of the zoo does: a call that cannot tell which closure it
// runs would otherwise reach the closures of the other cells (the candidates are the closures with the same number of arguments).
pub struct Held(pub String);

fn call(f: impl Fn(&str, &str) -> Held) -> String {
    f("", "").0
}

macro_rules! run_captured {
    ($e:expr) => {{
        let v = $e;
        call(move |_, _| Held(v.clone()))
    }};
}

fn vuln() { // @vuln
    let a = run_captured!("constant".to_string());
    let b = run_captured!(m::src("rust-macro-closure-twice-vuln")); // @source(rust-macro-closure-twice-vuln)
    m::sink_cmd(&a); // @sink(none)
    m::sink_cmd(&b); // @sink(rust-macro-closure-twice-vuln)
}

fn leak(id: &str) {
    m::sink_cmd(id); // @sink(rust-macro-closure-twice-vuln2)
}

macro_rules! route {
    ($r:expr, $path:expr) => {
        $r.get($path, |_req: &Request, id: &str| { // @source(rust-macro-closure-twice-vuln2)
            leak(id);
        })
    };
}

fn vuln2() { // @vuln
    let mut router = Router::new();
    route!(router, "/a/{id}");
    route!(router, "/b/{id}");
    let req = Request { path: "/a/x".to_string() };
    router.serve("/a/{id}", &req, &m::In::new("rust-macro-closure-twice-vuln2").value);
}

fn fixed() { // @fixed
    let a = run_captured!("constant".to_string());
    let b = run_captured!(m::clean_cmd(&m::src("rust-macro-closure-twice-fixed")));
    m::sink_cmd(&a); // @sink(none)
    m::sink_cmd(&b); // @sink(none)
}

fn decoy() { // @decoy
    let a = run_captured!(m::src("rust-macro-closure-twice-decoy")); // @source(rust-macro-closure-twice-decoy)
    let b = run_captured!("constant".to_string());
    m::sink_cmd(&b); // @sink(none) @nosink(rust-macro-closure-twice-decoy)
    m::sink_sql(&a); // @sink(rust-macro-closure-twice-decoy)
}
