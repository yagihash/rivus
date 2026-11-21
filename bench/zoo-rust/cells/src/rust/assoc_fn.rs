// @cell(F/rust/assoc-fn)
// @shape: the value comes from a function without a receiver (S::read(), T::read() of a trait through a type parameter) and the caller passes it to a sink (R-17)
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/assoc-fn/vuln", vuln);
    m::case("F/rust/assoc-fn/fixed", fixed);
    m::case("F/rust/assoc-fn/decoy", decoy);
    m::case("F/rust/assoc-fn/vuln2", vuln2);
}

trait Reader {
    fn read() -> String;
}

struct FromSrc;

impl Reader for FromSrc {
    fn read() -> String {
        m::src("rust-assoc-fn-vuln") // @source(rust-assoc-fn-vuln) @miss(unmodeled_library_call) @ref(R-17)
    }
}

fn use_it<T: Reader>() {
    m::sink_cmd(&T::read()); // @sink(rust-assoc-fn-vuln)
}

fn vuln() { // @vuln
    use_it::<FromSrc>();
}

struct Own;

impl Own {
    fn read() -> String {
        m::src("rust-assoc-fn-vuln2") // @source(rust-assoc-fn-vuln2)
    }
}

fn vuln2() { // @vuln
    m::sink_cmd(&Own::read()); // @sink(rust-assoc-fn-vuln2)
}

struct FromSrcFixed;

impl Reader for FromSrcFixed {
    fn read() -> String {
        m::clean_cmd(&m::src("rust-assoc-fn-fixed"))
    }
}

fn use_fixed<T: Reader>() {
    m::sink_cmd(&T::read()); // @sink(none)
}

fn fixed() { // @fixed
    use_fixed::<FromSrcFixed>();
}

struct Loud;

impl Reader for Loud {
    fn read() -> String {
        m::src("rust-assoc-fn-decoy") // @source(rust-assoc-fn-decoy) @miss(unmodeled_library_call) @ref(R-17)
    }
}

struct Calm;

impl Reader for Calm {
    fn read() -> String {
        "constant".to_string()
    }
}

fn quiet<T: Reader>() {
    m::sink_cmd(&T::read()); // @sink(none) @nosink(rust-assoc-fn-decoy)
}

fn loud<T: Reader>() {
    m::sink_sql(&T::read()); // @sink(rust-assoc-fn-decoy)
}

fn decoy() { // @decoy
    quiet::<Calm>();
    loud::<Loud>();
}
