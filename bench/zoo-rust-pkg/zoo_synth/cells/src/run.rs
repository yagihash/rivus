// @cell(F/pkg/zoo-synth/run)
// @shape: a value of a source goes to a function of a crate of another package (a skeleton) that the rules have as a sink; the skeleton records it
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/zoo-synth/run/vuln", vuln);
    m::case("F/pkg/zoo-synth/run/fixed", fixed);
    m::case("F/pkg/zoo-synth/run/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-zoo-synth-run-vuln"); // @source(pkg-zoo-synth-run-vuln)
    zoosynth::run(&v); // @sink(pkg-zoo-synth-run-vuln)
}

fn fixed() { // @fixed
    zoosynth::run(&m::clean_cmd(&m::src("pkg-zoo-synth-run-fixed"))); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-zoo-synth-run-decoy"); // @source(pkg-zoo-synth-run-decoy)
    zoosynth::run("constant"); // @sink(none) @nosink(pkg-zoo-synth-run-decoy)
    zoosynth::run(&t); // @sink(pkg-zoo-synth-run-decoy)
}
