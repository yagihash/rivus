// @cell(F/conv/builtin-minmax)
// @shape: std::cmp::min / max of strings give one of their arguments: the value that is the result
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/builtin-minmax/vuln", vuln);
    m::case("F/conv/builtin-minmax/fixed", fixed);
    m::case("F/conv/builtin-minmax/decoy", decoy);
}

fn vuln() { // @vuln
    let s = m::src("conv-builtin-minmax-vuln"); // @source(conv-builtin-minmax-vuln)
    m::sink_cmd(&std::cmp::min(s, "zzz".to_string())); // @sink(conv-builtin-minmax-vuln)
}

fn fixed() { // @fixed
    let s = m::clean_cmd(&m::src("conv-builtin-minmax-fixed"));
    m::sink_cmd(&std::cmp::max(String::new(), s)); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-builtin-minmax-decoy"); // @source(conv-builtin-minmax-decoy)
    m::sink_cmd(&std::cmp::min("b".to_string(), "a".to_string())); // @sink(none) @nosink(conv-builtin-minmax-decoy)
    m::sink_sql(&std::cmp::min(a, "c".to_string())); // @sink(conv-builtin-minmax-decoy)
}
