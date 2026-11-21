// @cell(F/rust/from-into)
// @shape: the value goes through From / Into (a struct made with .into(), an error converted by the ? operator)
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/from-into/vuln", vuln);
    m::case("F/rust/from-into/fixed", fixed);
    m::case("F/rust/from-into/decoy", decoy);
    m::case("F/rust/from-into/vuln2", vuln2);
}

struct Cmd {
    v: String,
}

impl From<String> for Cmd {
    fn from(s: String) -> Cmd {
        Cmd { v: s }
    }
}

fn vuln() { // @vuln
    let c: Cmd = m::src("rust-from-into-vuln").into(); // @source(rust-from-into-vuln)
    m::sink_cmd(&c.v); // @sink(rust-from-into-vuln)
}

#[derive(Debug)]
struct MyErr(String);

impl From<String> for MyErr {
    fn from(s: String) -> MyErr {
        MyErr(s)
    }
}

fn fail(s: String) -> Result<(), String> {
    if s.is_empty() {
        Ok(())
    } else {
        Err(s)
    }
}

fn run(s: String) -> Result<(), MyErr> {
    fail(s)?;
    Ok(())
}

fn vuln2() { // @vuln
    run(String::new()).unwrap();
    let e = run(m::src("rust-from-into-vuln2")).unwrap_err(); // @source(rust-from-into-vuln2)
    m::sink_cmd(&e.0); // @sink(rust-from-into-vuln2)
}

fn fixed() { // @fixed
    let c: Cmd = m::clean_cmd(&m::src("rust-from-into-fixed")).into();
    m::sink_cmd(&c.v); // @sink(none)
}

fn decoy() { // @decoy
    let a: Cmd = m::src("rust-from-into-decoy").into(); // @source(rust-from-into-decoy)
    let b: Cmd = "constant".to_string().into();
    m::sink_cmd(&b.v); // @sink(none) @nosink(rust-from-into-decoy)
    m::sink_sql(&a.v); // @sink(rust-from-into-decoy)
}
