// @cell(F/rust/iterator-chain)
// @shape: the value goes through an iterator chain (into_iter().map().filter().collect(), for_each into a captured String)
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/iterator-chain/vuln", vuln);
    m::case("F/rust/iterator-chain/fixed", fixed);
    m::case("F/rust/iterator-chain/decoy", decoy);
    m::case("F/rust/iterator-chain/vuln2", vuln2);
}

fn vuln() { // @vuln
    let v: Vec<String> = vec![m::src("rust-iterator-chain-vuln")].into_iter().map(|s| s).filter(|s| !s.is_empty()).collect(); // @source(rust-iterator-chain-vuln)
    m::sink_cmd(&v[0]); // @sink(rust-iterator-chain-vuln)
}

fn vuln2() { // @vuln
    let mut out = String::new();
    [m::src("rust-iterator-chain-vuln2")].iter().for_each(|x| out.push_str(x)); // @source(rust-iterator-chain-vuln2) @miss(unmodeled_library_call) @ref(ENV-PTR)
    m::sink_cmd(&out); // @sink(rust-iterator-chain-vuln2)
}

fn fixed() { // @fixed
    let v: Vec<String> = vec![m::clean_cmd(&m::src("rust-iterator-chain-fixed"))].into_iter().map(|s| s).filter(|s| !s.is_empty()).collect();
    m::sink_cmd(&v[0]); // @sink(none)
}

fn decoy() { // @decoy
    let a: Vec<String> = vec![m::src("rust-iterator-chain-decoy")].into_iter().map(|s| s).collect(); // @source(rust-iterator-chain-decoy)
    let b: Vec<String> = vec!["constant".to_string()].into_iter().map(|s| s).collect();
    m::sink_cmd(&b[0]); // @sink(none) @nosink(rust-iterator-chain-decoy)
    m::sink_sql(&a[0]); // @sink(rust-iterator-chain-decoy)
}
