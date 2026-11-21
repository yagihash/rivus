// @cell(F/rust/option-result-chain)
// @shape: the value goes through Option and Result (map, and_then, unwrap_or_default, the ? operator)
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/option-result-chain/vuln", vuln);
    m::case("F/rust/option-result-chain/fixed", fixed);
    m::case("F/rust/option-result-chain/decoy", decoy);
    m::case("F/rust/option-result-chain/vuln2", vuln2);
}

fn vuln() { // @vuln
    let r = Some(m::src("rust-option-result-chain-vuln")).map(|s| s).and_then(Some).unwrap_or_default(); // @source(rust-option-result-chain-vuln)
    m::sink_cmd(&r); // @sink(rust-option-result-chain-vuln)
}

fn pass(s: String, ok: bool) -> Result<String, String> {
    if ok {
        Ok(s)
    } else {
        Err("no".to_string())
    }
}

fn through(s: String, ok: bool) -> Result<String, String> {
    let t = pass(s, ok)?;
    Ok(t)
}

fn vuln2() { // @vuln
    let v = m::src("rust-option-result-chain-vuln2"); // @source(rust-option-result-chain-vuln2)
    let _ = through(v.clone(), false);
    let r = through(v, true).unwrap();
    m::sink_cmd(&r); // @sink(rust-option-result-chain-vuln2)
}

fn fixed() { // @fixed
    let r = Some(m::clean_cmd(&m::src("rust-option-result-chain-fixed"))).map(|s| s).and_then(Some).unwrap_or_default();
    m::sink_cmd(&r); // @sink(none)
}

fn decoy() { // @decoy
    let a = Some(m::src("rust-option-result-chain-decoy")); // @source(rust-option-result-chain-decoy)
    let b = Some("constant".to_string());
    m::sink_cmd(&b.map(|s| s).unwrap_or_default()); // @sink(none) @nosink(rust-option-result-chain-decoy)
    m::sink_sql(&a.map(|s| s).unwrap_or_default()); // @sink(rust-option-result-chain-decoy)
}
