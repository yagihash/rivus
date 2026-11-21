// @cell(F/rust/cfg-feature)
// @shape: the flow is in code that is compiled only with a feature that is not a default one (R-17: the analysis reads the default build; the harness builds with the feature)
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/cfg-feature/vuln", vuln);
    m::case("F/rust/cfg-feature/fixed", fixed);
    m::case("F/rust/cfg-feature/decoy", decoy);
}

fn vuln() { // @vuln
    #[cfg(feature = "extra")]
    {
        let v = m::src("rust-cfg-feature-vuln"); // @source(rust-cfg-feature-vuln) @miss(silent) @ref(R-17)
        m::sink_cmd(&v); // @sink(rust-cfg-feature-vuln)
    }
}

fn fixed() { // @fixed
    #[cfg(feature = "extra")]
    {
        let v = m::clean_cmd(&m::src("rust-cfg-feature-fixed"));
        m::sink_cmd(&v); // @sink(none)
    }
}

fn decoy() { // @decoy
    #[cfg(feature = "extra")]
    {
        let a = m::src("rust-cfg-feature-decoy"); // @source(rust-cfg-feature-decoy) @miss(silent) @ref(R-17)
        m::sink_cmd("constant"); // @sink(none) @nosink(rust-cfg-feature-decoy)
        m::sink_sql(&a); // @sink(rust-cfg-feature-decoy)
    }
}
