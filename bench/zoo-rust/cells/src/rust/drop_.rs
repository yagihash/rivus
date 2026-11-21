// @cell(F/rust/drop)
// @shape: the value is held by a type that implements Drop, and the sink is in drop(): a local that goes out of scope, a field of another struct, an Option, a Box, an explicit drop() (R-17)
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/drop/vuln", vuln);
    m::case("F/rust/drop/fixed", fixed);
    m::case("F/rust/drop/decoy", decoy);
    m::case("F/rust/drop/vuln2", vuln2);
    m::case("F/rust/drop/vuln3", vuln3);
    m::case("F/rust/drop/vuln4", vuln4);
    m::case("F/rust/drop/vuln5", vuln5);
}

struct G1(String);
impl Drop for G1 {
    fn drop(&mut self) {
        m::sink_cmd(&self.0); // @sink(rust-drop-vuln)
    }
}

fn vuln() { // @vuln
    let _g = G1(m::src("rust-drop-vuln")); // @source(rust-drop-vuln)
}

struct G2(String);
impl Drop for G2 {
    fn drop(&mut self) {
        m::sink_cmd(&self.0); // @sink(rust-drop-vuln2)
    }
}
struct Outer2 {
    _g: G2,
}

fn vuln2() { // @vuln
    let _o = Outer2 { _g: G2(m::src("rust-drop-vuln2")) }; // @source(rust-drop-vuln2) @miss(unmodeled_library_call) @ref(R-17)
}

struct G3(String);
impl Drop for G3 {
    fn drop(&mut self) {
        m::sink_cmd(&self.0); // @sink(rust-drop-vuln3)
    }
}

fn vuln3() { // @vuln
    let _o = Some(G3(m::src("rust-drop-vuln3"))); // @source(rust-drop-vuln3) @miss(unmodeled_library_call) @ref(R-17)
}

struct G4(String);
impl Drop for G4 {
    fn drop(&mut self) {
        m::sink_cmd(&self.0); // @sink(rust-drop-vuln4)
    }
}

fn vuln4() { // @vuln
    let _b = Box::new(G4(m::src("rust-drop-vuln4"))); // @source(rust-drop-vuln4) @miss(unmodeled_library_call) @ref(R-17)
}

struct G5(String);
impl Drop for G5 {
    fn drop(&mut self) {
        m::sink_cmd(&self.0); // @sink(rust-drop-vuln5)
    }
}

fn vuln5() { // @vuln
    let g = G5(m::src("rust-drop-vuln5")); // @source(rust-drop-vuln5) @miss(unmodeled_library_call) @ref(R-17)
    drop(g);
}

struct GF(String);
impl Drop for GF {
    fn drop(&mut self) {
        m::sink_cmd(&self.0); // @sink(none)
    }
}

fn fixed() { // @fixed
    let _g = GF(m::clean_cmd(&m::src("rust-drop-fixed")));
}

struct Quiet(String);
impl Drop for Quiet {
    fn drop(&mut self) {
        m::sink_cmd(&self.0); // @sink(none) @nosink(rust-drop-decoy)
    }
}
struct Loud(String);
impl Drop for Loud {
    fn drop(&mut self) {
        m::sink_sql(&self.0); // @sink(rust-drop-decoy)
    }
}

fn decoy() { // @decoy
    let _q = Quiet("constant".to_string());
    let _l = Loud(m::src("rust-drop-decoy")); // @source(rust-drop-decoy)
}
