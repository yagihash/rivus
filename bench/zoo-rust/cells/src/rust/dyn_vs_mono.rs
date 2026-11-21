// @cell(F/rust/dyn-vs-mono)
// @shape: the same trait is used through a &dyn (dynamic dispatch, vuln) and through a type parameter (static, monomorphized, vuln2)
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/dyn-vs-mono/vuln", vuln);
    m::case("F/rust/dyn-vs-mono/fixed", fixed);
    m::case("F/rust/dyn-vs-mono/decoy", decoy);
    m::case("F/rust/dyn-vs-mono/vuln2", vuln2);
}

trait Getter {
    fn get(&self) -> String;
}

struct Own {
    v: String,
}

impl Getter for Own {
    fn get(&self) -> String {
        self.v.clone()
    }
}

fn use_dyn(g: &dyn Getter) {
    m::sink_cmd(&g.get()); // @sink(rust-dyn-vs-mono-vuln)
}

fn use_mono<T: Getter>(g: &T) {
    m::sink_cmd(&g.get()); // @sink(rust-dyn-vs-mono-vuln2)
}

fn vuln() { // @vuln
    use_dyn(&Own { v: m::src("rust-dyn-vs-mono-vuln") }); // @source(rust-dyn-vs-mono-vuln)
}

fn vuln2() { // @vuln
    use_mono(&Own { v: m::src("rust-dyn-vs-mono-vuln2") }); // @source(rust-dyn-vs-mono-vuln2)
}

fn use_dyn_fixed(g: &dyn Getter) {
    m::sink_cmd(&g.get()); // @sink(none)
}

fn use_mono_fixed<T: Getter>(g: &T) {
    m::sink_cmd(&g.get()); // @sink(none)
}

fn fixed() { // @fixed
    use_dyn_fixed(&Own { v: m::clean_cmd(&m::src("rust-dyn-vs-mono-fixed")) });
    use_mono_fixed(&Own { v: m::clean_cmd(&m::src("rust-dyn-vs-mono-fixed-b")) });
}

struct Plain;

impl Getter for Plain {
    fn get(&self) -> String {
        "constant".to_string()
    }
}

fn quiet_dyn(g: &dyn Getter) {
    m::sink_cmd(&g.get()); // @sink(none) @nosink(rust-dyn-vs-mono-decoy)
}

fn loud_mono<T: Getter>(g: &T) {
    m::sink_sql(&g.get()); // @sink(rust-dyn-vs-mono-decoy)
}

fn decoy() { // @decoy
    quiet_dyn(&Plain);
    loud_mono(&Own { v: m::src("rust-dyn-vs-mono-decoy") }); // @source(rust-dyn-vs-mono-decoy)
}
