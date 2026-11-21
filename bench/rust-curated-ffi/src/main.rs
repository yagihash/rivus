// A program that starts at `main` is read as a closed world: a `pub` helper has no caller that is not read. This one has
// a function that another program can call by its symbol (`#[no_mangle]`), so the program is called from outside, and what
// is `pub` is not closed: the candidates of a call of a closure in `run_pub` stay (docs/DESIGN-capture.md). A helper that
// is private to the crate (`run_priv`) has all its callers in the crate, with or without that.

fn source() -> String {
    String::from("x")
}

fn sink(_s: &str) {}

#[no_mangle]
pub extern "C" fn entry_from_outside() -> i32 {
    0
}

struct Pe(String);

pub fn run_pub(f: impl Fn(&Pe), s: &Pe) {
    f(s)
}

fn pub_a() {
    let a = Pe(source()); // @source(ffi-pub)
    run_pub(
        |x| {
            sink(&x.0); // @sink(ffi-pub)
        },
        &a,
    );
}

fn pub_b() {
    run_pub(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &Pe(String::from("constant")),
    );
}

struct Pf(String);

fn run_priv(f: impl Fn(&Pf), s: &Pf) {
    f(s)
}

fn priv_a() {
    let a = Pf(source()); // @source(ffi-priv)
    run_priv(
        |x| {
            sink(&x.0); // @sink(ffi-priv)
        },
        &a,
    );
}

fn priv_b() {
    run_priv(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &Pf(String::from("constant")),
    );
}

fn main() {
    pub_a();
    pub_b();
    priv_a();
    priv_b();
}
