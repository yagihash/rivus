// @cell(F/cont/ptr-commaok-assert)
// @shape: a `&mut T` taken out of a `&mut dyn Any` by `downcast_mut::<T>()`, and a field written through it, is read from the owner again
use std::any::Any;
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-commaok-assert/vuln", vuln);
    m::case("F/cont/ptr-commaok-assert/fixed", fixed);
    m::case("F/cont/ptr-commaok-assert/decoy", decoy);
}

struct Boxed {
    v: String,
}

fn vuln() { // @vuln
    let mut owner = Boxed { v: String::new() };
    let x: &mut dyn Any = &mut owner;
    let p = x.downcast_mut::<Boxed>().unwrap();
    p.v = m::src("cont-ptr-commaok-assert-vuln"); // @source(cont-ptr-commaok-assert-vuln)
    m::sink_cmd(&owner.v); // @sink(cont-ptr-commaok-assert-vuln)
}

fn fixed() { // @fixed
    let mut owner = Boxed { v: String::new() };
    let x: &mut dyn Any = &mut owner;
    let p = x.downcast_mut::<Boxed>().unwrap();
    p.v = m::clean_cmd(&m::src("cont-ptr-commaok-assert-fixed"));
    m::sink_cmd(&owner.v); // @sink(none)
}

fn decoy() { // @decoy
    let mut owner = Boxed { v: String::new() };
    let other = Boxed { v: String::new() };
    let x: &mut dyn Any = &mut owner;
    let p = x.downcast_mut::<Boxed>().unwrap();
    p.v = m::src("cont-ptr-commaok-assert-decoy"); // @source(cont-ptr-commaok-assert-decoy)
    m::sink_cmd(&other.v); // @sink(none)
    m::sink_sql(&owner.v); // @sink(cont-ptr-commaok-assert-decoy)
}
