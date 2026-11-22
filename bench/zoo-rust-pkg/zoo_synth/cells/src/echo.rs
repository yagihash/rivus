// @cell(F/pkg/zoo-synth/echo)
// @shape: a value of a source goes through a function of a crate of another package (a skeleton) to a sink
use zoo_mark as m;
use zoosynth::echo;

pub fn register() {
    m::case("F/pkg/zoo-synth/echo/vuln", vuln);
    m::case("F/pkg/zoo-synth/echo/fixed", fixed);
    m::case("F/pkg/zoo-synth/echo/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-zoo-synth-echo-vuln"); // @source(pkg-zoo-synth-echo-vuln)
    m::sink_cmd(&echo(&v)); // @sink(pkg-zoo-synth-echo-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&echo(&m::src("pkg-zoo-synth-echo-fixed")));
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-zoo-synth-echo-decoy"); // @source(pkg-zoo-synth-echo-decoy)
    m::sink_cmd(&echo("constant")); // @sink(none) @nosink(pkg-zoo-synth-echo-decoy)
    m::sink_cmd(&echo(&t)); // @sink(pkg-zoo-synth-echo-decoy)
}
