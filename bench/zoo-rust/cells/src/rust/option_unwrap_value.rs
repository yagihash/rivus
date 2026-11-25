// @cell(F/rust/option-unwrap-value)
// @shape: values taken out of an Option and out of an iterator by value (unwrap, next, into_iter, as_mut) are written to, and the Option or Vec they came from is read again
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/option-unwrap-value/vuln", vuln);
    m::case("F/rust/option-unwrap-value/fixed", fixed);
    m::case("F/rust/option-unwrap-value/decoy", decoy);
    m::case("F/rust/option-unwrap-value/vuln2", vuln2);
}

fn vuln() { // @vuln
    let mut o = Some(String::new());
    o.as_mut().unwrap().push_str(&m::src("rust-option-unwrap-value-vuln")); // @source(rust-option-unwrap-value-vuln)
    m::sink_cmd(&o.unwrap()); // @sink(rust-option-unwrap-value-vuln)
}

fn vuln2() { // @vuln
    let mut xs = vec![String::new()];
    let mut it = xs.iter_mut();
    let p = it.next().unwrap();
    p.push_str(&m::src("rust-option-unwrap-value-vuln2")); // @source(rust-option-unwrap-value-vuln2)
    m::sink_cmd(&xs[0]); // @sink(rust-option-unwrap-value-vuln2)
}

fn fixed() { // @fixed
    let mut o = Some(String::new());
    o.as_mut().unwrap().push_str(&m::clean_cmd(&m::src("rust-option-unwrap-value-fixed")));
    m::sink_cmd(&o.unwrap()); // @sink(none)
}

fn decoy() { // @decoy
    let a = Some(String::new());
    let b = Some(String::new());
    let mut s = a.unwrap();
    s.push_str(&m::src("rust-option-unwrap-value-decoy")); // @source(rust-option-unwrap-value-decoy)
    m::sink_cmd(&b.unwrap()); // @sink(none)
    m::sink_sql(&s); // @sink(rust-option-unwrap-value-decoy)
    decoy2();
    decoy3();
}

fn decoy2() {
    let o = Some(String::from("constant"));
    let mut x = o.clone().unwrap();
    x.push_str(&m::src("rust-option-unwrap-value-decoy2")); // @source(rust-option-unwrap-value-decoy2)
    let y = o.clone().unwrap();
    m::sink_cmd(&o.unwrap()); // @sink(none)
    m::sink_cmd(&y); // @sink(none)
    m::sink_sql(&x); // @sink(rust-option-unwrap-value-decoy2)
}

fn decoy3() {
    let xs = vec![String::from("constant")];
    let ys = vec![String::new()];
    let mut it = ys.into_iter();
    let mut s = it.next().unwrap();
    s.push_str(&m::src("rust-option-unwrap-value-decoy3")); // @source(rust-option-unwrap-value-decoy3)
    for c in xs.into_iter() {
        m::sink_cmd(&c); // @sink(none)
    }
    m::sink_sql(&s); // @sink(rust-option-unwrap-value-decoy3)
}
