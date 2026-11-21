// @cell(F/rust/channel)
// @shape: a value is sent on an mpsc channel and received by the other end
use std::sync::mpsc;
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/channel/vuln", vuln);
    m::case("F/rust/channel/fixed", fixed);
    m::case("F/rust/channel/decoy", decoy);
}

fn vuln() { // @vuln
    let (tx, rx) = mpsc::channel();
    tx.send(m::src("rust-channel-vuln")).unwrap(); // @source(rust-channel-vuln) @miss(unmodeled_library_call) @ref(STD-MODEL)
    let v: String = rx.recv().unwrap();
    m::sink_cmd(&v); // @sink(rust-channel-vuln)
}

fn fixed() { // @fixed
    let (tx, rx) = mpsc::channel();
    tx.send(m::clean_cmd(&m::src("rust-channel-fixed"))).unwrap();
    let v: String = rx.recv().unwrap();
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let (tx_a, rx_a) = mpsc::channel();
    let (tx_b, rx_b) = mpsc::channel();
    tx_a.send(m::src("rust-channel-decoy")).unwrap(); // @source(rust-channel-decoy) @miss(unmodeled_library_call) @ref(STD-MODEL)
    tx_b.send("constant".to_string()).unwrap();
    let b: String = rx_b.recv().unwrap();
    let a: String = rx_a.recv().unwrap();
    m::sink_cmd(&b); // @sink(none) @nosink(rust-channel-decoy)
    m::sink_sql(&a); // @sink(rust-channel-decoy)
}
