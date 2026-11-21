// @cell(F/clos/partial-callers)
// @shape: one helper calls the function it is given; one caller gives a function made right there, another a function it reads from a field (a caller whose value cannot be followed)
use zoo_mark as m;

// Each case has its own argument type, so that the cases of the cell do not meet at a call of a function pointer.
struct Tv(String);
struct Tf(String);
struct Td(String);

struct Hv {
    cb: fn(&Tv),
}
struct Hf {
    cb: fn(&Tf),
}
struct Hd {
    cb: fn(&Td),
}

pub fn register() {
    m::case("F/clos/partial-callers/vuln", vuln);
    m::case("F/clos/partial-callers/fixed", fixed);
    m::case("F/clos/partial-callers/decoy", decoy);
}

fn apply_vuln(f: fn(&Tv), s: &Tv) {
    f(s)
}

fn vuln() { // @vuln
    let v = Tv(m::src("clos-partial-callers-vuln")); // @source(clos-partial-callers-vuln) @miss(unmodeled_library_call) @ref(PARTIAL-CALLERS,PARITY)
    apply_vuln(|_x| {}, &Tv("constant".to_string()));
    let h = Hv {
        cb: |x| {
            m::sink_cmd(&x.0); // @sink(clos-partial-callers-vuln)
        },
    };
    apply_vuln(h.cb, &v);
}

fn apply_fixed(f: fn(&Tf), s: &Tf) {
    f(s)
}

fn fixed() { // @fixed
    let v = Tf(m::clean_cmd(&m::src("clos-partial-callers-fixed")));
    apply_fixed(|_x| {}, &Tf("constant".to_string()));
    let h = Hf {
        cb: |x| {
            m::sink_cmd(&x.0); // @sink(none)
        },
    };
    apply_fixed(h.cb, &v);
}

fn apply_decoy(f: fn(&Td), s: &Td) {
    f(s)
}

fn decoy() { // @decoy
    let v = Td(m::src("clos-partial-callers-decoy")); // @source(clos-partial-callers-decoy) @miss(unmodeled_library_call) @ref(PARTIAL-CALLERS,PARITY)
    let h = Hd {
        cb: |x| {
            m::sink_cmd(&x.0); // @sink(none)
        },
    };
    apply_decoy(h.cb, &Td("constant".to_string()));
    apply_decoy(
        |x| {
            m::sink_sql(&x.0); // @sink(clos-partial-callers-decoy)
        },
        &v,
    );
}
