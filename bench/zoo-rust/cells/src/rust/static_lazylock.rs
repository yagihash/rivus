// @cell(F/rust/static-lazylock)
// @shape: a static LazyLock<String> is initialised from a source and read by another function (R-17)
use std::sync::LazyLock;
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/static-lazylock/vuln", vuln);
    m::case("F/rust/static-lazylock/fixed", fixed);
    m::case("F/rust/static-lazylock/decoy", decoy);
}

static CMD: LazyLock<String> = LazyLock::new(|| m::src("rust-static-lazylock-vuln")); // @source(rust-static-lazylock-vuln) @miss(silent) @ref(R-17)

fn read() {
    m::sink_cmd(&CMD); // @sink(rust-static-lazylock-vuln)
}

fn vuln() { // @vuln
    read();
}

static CLEAN: LazyLock<String> = LazyLock::new(|| m::clean_cmd(&m::src("rust-static-lazylock-fixed")));

fn read_fixed() {
    m::sink_cmd(&CLEAN); // @sink(none)
}

fn fixed() { // @fixed
    read_fixed();
}

static LOUD: LazyLock<String> = LazyLock::new(|| m::src("rust-static-lazylock-decoy")); // @source(rust-static-lazylock-decoy) @miss(silent) @ref(R-17)
static CALM: LazyLock<String> = LazyLock::new(|| "constant".to_string());

fn read_decoy() {
    m::sink_cmd(&CALM); // @sink(none) @nosink(rust-static-lazylock-decoy)
    m::sink_sql(&LOUD); // @sink(rust-static-lazylock-decoy)
}

fn decoy() { // @decoy
    read_decoy();
}
