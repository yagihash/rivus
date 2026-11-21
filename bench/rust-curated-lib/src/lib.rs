// A library with no `main`: the program that uses it is not read, so what is `pub` here may be called by code that is
// not (the open world). The candidates of a call of a closure are narrowed to the closures that flow to the receiver
// only when every caller of the helper is known: a `pub` helper of a library is not (`run_pub`), one that is private to
// the crate is (`run_priv`: only this crate can call it). Docs: docs/DESIGN-capture.md.

pub fn source() -> String {
    String::from("x")
}

pub fn sink(_s: &str) {}

pub struct Pe(pub String);

// `pub`: a caller outside may give a closure of any type that fits, so every candidate stays
pub fn run_pub(f: impl Fn(&Pe), s: &Pe) {
    f(s)
}

pub fn pub_a() {
    let a = Pe(source()); // @source(open-pub)
    run_pub(
        |x| {
            sink(&x.0); // @sink(open-pub)
        },
        &a,
    );
}

pub fn pub_b() {
    run_pub(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &Pe(String::from("constant")),
    );
}

pub struct Pf(pub String);

// private: only this crate calls it, and all its callers are read: the closure that is passed is the one that is called
fn run_priv(f: impl Fn(&Pf), s: &Pf) {
    f(s)
}

pub fn priv_a() {
    let a = Pf(source()); // @source(open-priv)
    run_priv(
        |x| {
            sink(&x.0); // @sink(open-priv)
        },
        &a,
    );
}

pub fn priv_b() {
    run_priv(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &Pf(String::from("constant")),
    );
}

pub struct Pg(pub String);

// a default method of a `pub` trait has no `pub` in front of it, and is as public as the trait: a caller outside may
// give a closure of any type that fits, so every candidate stays
pub trait Each {
    fn each(&self, f: impl Fn(&Pg), s: &Pg) {
        f(s)
    }
}

pub struct Lister;

impl Each for Lister {}

pub fn trait_a() {
    let a = Pg(source()); // @source(open-trait)
    Lister.each(
        |x| {
            sink(&x.0); // @sink(open-trait)
        },
        &a,
    );
}

pub fn trait_b() {
    Lister.each(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &Pg(String::from("constant")),
    );
}

pub struct Ph(pub String);

// a default method of a trait that is private to the crate: only this crate calls it, and the callers are all read
trait EachPriv {
    fn each_priv(&self, f: impl Fn(&Ph), s: &Ph) {
        f(s)
    }
}

impl EachPriv for Lister {}

pub fn trait_priv_a() {
    let a = Ph(source()); // @source(open-trait-priv)
    Lister.each_priv(
        |x| {
            sink(&x.0); // @sink(open-trait-priv)
        },
        &a,
    );
}

pub fn trait_priv_b() {
    Lister.each_priv(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &Ph(String::from("constant")),
    );
}
