use crate::{sink, source};

// The candidates of a call of a closure (`Fn::call`) are every closure whose arguments have the types of the
// call, and are narrowed to the closures that flow to the receiver, when every one of them can be followed (the
// receiver is made here, or is a parameter whose callers are all read; docs/DESIGN-capture.md). When one cannot be,
// every candidate stays, and the flows through all of them are found as before: the forms below are the ones
// where narrowing would lose a flow, and each is a `@sink` that has to stay found. Each form has types of its own
// (a newtype of String), so that the closures of the other files are not its candidates.

// (1) two callers, one of them gives what cannot be followed (a closure that captured a value, returned by a
// function: it is the result of a call. A closure that captured nothing is not: its value is made of its
// type, and is the closure): the candidates stay, and the closure that `make_unknown` returns still gets the
// value. The other closure is a candidate too, so the flow to its sink is the price of not knowing (`@fp`)
struct Pa(String);

fn run_pa(f: impl Fn(&Pa), s: &Pa) {
    f(s)
}

fn make_unknown(tag: String) -> impl Fn(&Pa) {
    move |x| {
        sink(&x.0); // @sink(narrow-unknown)
        sink(&tag);
    }
}

pub fn two_origins_known() {
    run_pa(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &Pa(String::from("constant")),
    );
}

pub fn two_origins_unknown() {
    let a = Pa(source()); // @source(narrow-unknown)
    run_pa(make_unknown(String::from("tag")), &a);
}

// (2) the closure is kept in a field of a struct and another function calls it: the receiver comes from a
// field, which is not followed, so the candidates stay
struct Pb(String);

struct Holder<F: Fn(&Pb)> {
    f: F,
}

fn call_field<F: Fn(&Pb)>(h: &Holder<F>, s: &Pb) {
    (h.f)(s)
}

pub fn through_field() {
    let a = Pb(source()); // @source(narrow-field)
    let h = Holder {
        f: |x: &Pb| {
            sink(&x.0); // @sink(narrow-field)
        },
    };
    call_field(&h, &a);
}

pub fn other_closure_of_the_type() {
    let b = Pb(String::from("constant"));
    let g = |x: &Pb| {
        sink(&x.0); // @sink(none) @fp
    };
    g(&b);
}

// (3) the closure is a parameter of the helper that a caller passes on (a relay): the callers of the helper
// are all read, so it is followed. The guard of the dispatch (which closure the caller gave) does not pass through
// the relay, so the other caller's sink is reached all the same (`@fp`, the same as the relay of the zoo)
struct Pc(String);

fn inner_pc(f: impl Fn(&Pc), s: &Pc) {
    f(s)
}

fn outer_pc(g: impl Fn(&Pc), s: &Pc) {
    inner_pc(g, s)
}

pub fn relay_known_a() {
    let a = Pc(source()); // @source(narrow-relay)
    outer_pc(
        |x| {
            sink(&x.0); // @sink(narrow-relay)
        },
        &a,
    );
}

pub fn relay_known_b() {
    outer_pc(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &Pc(String::from("constant")),
    );
}

// (4) 8 callers of one helper, and two more closures of the type that are not passed to it: the candidates are 10, and the
// functions that flow to the receiver are 8 (the edge of the limit of 8: narrowed)
struct P8(String);

fn run_lim8(f: impl Fn(&P8), s: &P8) {
    f(s)
}

pub fn lim8_1() {
    let a = P8(source()); // @source(lim8)
    run_lim8(
        |x| {
            sink(&x.0); // @sink(lim8)
        },
        &a,
    );
}

pub fn lim8_2() {
    run_lim8(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &P8(String::from("constant")),
    );
}

pub fn lim8_3() {
    run_lim8(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &P8(String::from("constant")),
    );
}

pub fn lim8_4() {
    run_lim8(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &P8(String::from("constant")),
    );
}

pub fn lim8_5() {
    run_lim8(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &P8(String::from("constant")),
    );
}

pub fn lim8_6() {
    run_lim8(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &P8(String::from("constant")),
    );
}

pub fn lim8_7() {
    run_lim8(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &P8(String::from("constant")),
    );
}

pub fn lim8_8() {
    run_lim8(
        |x| {
            sink(&x.0); // @sink(none)
        },
        &P8(String::from("constant")),
    );
}

pub fn lim8_extra_1() {
    let g = |x: &P8| {
        sink(&x.0); // @sink(none)
    };
    g(&P8(String::from("constant")));
}

pub fn lim8_extra_2() {
    let g = |x: &P8| {
        sink(&x.0); // @sink(none)
    };
    g(&P8(String::from("constant")));
}

// (4) 9 callers of one helper, and two more closures of the type that are not passed to it: the candidates are 11, and the
// functions that flow to the receiver are 9 (more than the limit of 8: not narrowed)
struct P9(String);

fn run_lim9(f: impl Fn(&P9), s: &P9) {
    f(s)
}

pub fn lim9_1() {
    let a = P9(source()); // @source(lim9)
    run_lim9(
        |x| {
            sink(&x.0); // @sink(lim9)
        },
        &a,
    );
}

pub fn lim9_2() {
    run_lim9(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &P9(String::from("constant")),
    );
}

pub fn lim9_3() {
    run_lim9(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &P9(String::from("constant")),
    );
}

pub fn lim9_4() {
    run_lim9(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &P9(String::from("constant")),
    );
}

pub fn lim9_5() {
    run_lim9(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &P9(String::from("constant")),
    );
}

pub fn lim9_6() {
    run_lim9(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &P9(String::from("constant")),
    );
}

pub fn lim9_7() {
    run_lim9(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &P9(String::from("constant")),
    );
}

pub fn lim9_8() {
    run_lim9(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &P9(String::from("constant")),
    );
}

pub fn lim9_9() {
    run_lim9(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &P9(String::from("constant")),
    );
}

pub fn lim9_extra_1() {
    let g = |x: &P9| {
        sink(&x.0); // @sink(none) @fp
    };
    g(&P9(String::from("constant")));
}

pub fn lim9_extra_2() {
    let g = |x: &P9| {
        sink(&x.0); // @sink(none) @fp
    };
    g(&P9(String::from("constant")));
}

// (10) the same two closures reach one call as the parameter 0 in one call of the helper and as the parameter 1
// in another: the closure that does the sink has to stay a candidate of the call, whichever parameter the walk
// went through first
struct Pz(String);

fn home_pz<F: Fn(&Pz), G: Fn(&Pz)>(f: F, g: G, c: bool, s: &Pz) {
    let h: &dyn Fn(&Pz) = if c { &f } else { &g };
    h(s)
}

pub fn param_order(flag: bool) {
    let a = |_x: &Pz| {};
    let b = |x: &Pz| {
        sink(&x.0); // @sink(narrow-param-order)
    };
    home_pz(a, b, flag, &Pz(source())); // @source(narrow-param-order)
    let a2 = |_x: &Pz| {};
    let b2 = |x: &Pz| {
        sink(&x.0); // @sink(none)
    };
    home_pz(b2, a2, flag, &Pz(String::from("constant")));
}
