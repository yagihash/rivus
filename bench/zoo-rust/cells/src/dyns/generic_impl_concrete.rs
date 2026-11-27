// @cell(F/dyn/generic-impl-concrete)
// @shape: a trait is implemented for a generic type (impl<T> Tr for Foo<T>) and for another type; a Foo<u8> (the type with its argument, not the generic one of the impl) is called through a &dyn Tr, and the value reaches a sink in the generic impl
use std::marker::PhantomData;
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/generic-impl-concrete/vuln", vuln);
    m::case("F/dyn/generic-impl-concrete/fixed", fixed);
    m::case("F/dyn/generic-impl-concrete/decoy", decoy);
}

trait TrV {
    fn run(&self, s: &str);
}

struct Foo<T>(PhantomData<T>);

impl<T> TrV for Foo<T> {
    fn run(&self, s: &str) {
        m::sink_cmd(s); // @sink(dyn-generic-impl-concrete-vuln)
    }
}

struct Other;

impl TrV for Other {
    fn run(&self, s: &str) {
        m::sink_cmd(&m::clean_cmd(s)); // @sink(none)
    }
}

fn vuln() { // @vuln
    let f: Foo<u8> = Foo(PhantomData);
    let d: &dyn TrV = &f;
    d.run(&m::src("dyn-generic-impl-concrete-vuln")); // @source(dyn-generic-impl-concrete-vuln)
    let o: &dyn TrV = &Other;
    o.run("constant");
}

trait TrF {
    fn run(&self, s: &str);
}

struct FixedFoo<T>(PhantomData<T>);

impl<T> TrF for FixedFoo<T> {
    fn run(&self, s: &str) {
        m::sink_cmd(&m::clean_cmd(s)); // @sink(none)
    }
}

struct OtherF;

impl TrF for OtherF {
    fn run(&self, s: &str) {
        m::sink_cmd(&m::clean_cmd(s)); // @sink(none)
    }
}

fn fixed() { // @fixed
    let f: FixedFoo<u8> = FixedFoo(PhantomData);
    let d: &dyn TrF = &f;
    d.run(&m::src("dyn-generic-impl-concrete-fixed"));
    let o: &dyn TrF = &OtherF;
    o.run("constant");
}

trait TrD {
    fn run(&self, s: &str);
    fn note(&self, s: &str);
}

struct DecoyFoo<T>(PhantomData<T>);

impl<T> TrD for DecoyFoo<T> {
    fn run(&self, s: &str) {
        m::sink_sql(s); // @sink(dyn-generic-impl-concrete-decoy)
    }
    fn note(&self, s: &str) {
        m::sink_cmd(s); // @sink(none) @nosink(dyn-generic-impl-concrete-decoy)
    }
}

fn decoy() { // @decoy
    let f: DecoyFoo<u8> = DecoyFoo(PhantomData);
    let d: &dyn TrD = &f;
    d.note("constant");
    d.run(&m::src("dyn-generic-impl-concrete-decoy")); // @source(dyn-generic-impl-concrete-decoy)
}
