// @cell(F/clos/local-recursion)
// @shape: a function declared inside another calls itself by its name, and the value reaches the sink only through the recursive call
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/local-recursion/vuln", vuln);
    m::case("F/clos/local-recursion/fixed", fixed);
    m::case("F/clos/local-recursion/decoy", decoy);
}

fn vuln() { // @vuln
    // the two arguments change places in each call: the first one reaches the sink in the second call
    fn rec(x: &str, y: &str, n: i32) {
        if n == 0 {
            m::sink_cmd(y); // @sink(clos-local-recursion-vuln)
            return;
        }
        rec(y, x, n - 1);
    }
    rec(&m::src("clos-local-recursion-vuln"), "constant", 1); // @source(clos-local-recursion-vuln)
}

fn fixed() { // @fixed
    fn rec(x: &str, y: &str, n: i32) {
        if n == 0 {
            m::sink_cmd(y); // @sink(none)
            return;
        }
        rec(y, x, n - 1);
    }
    rec(&m::clean_cmd(&m::src("clos-local-recursion-fixed")), "constant", 1);
}

fn decoy() { // @decoy
    fn rec(a: &str, b: &str, n: i32) {
        if n == 0 {
            m::sink_cmd(b); // @sink(none) @nosink(clos-local-recursion-decoy)
            m::sink_sql(a); // @sink(clos-local-recursion-decoy)
            return;
        }
        rec(a, b, n - 1);
    }
    rec(&m::src("clos-local-recursion-decoy"), "constant", 1); // @source(clos-local-recursion-decoy)
}
