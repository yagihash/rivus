// `zoo-observe`: runs every case once and writes what the sinks got (ZOO_OUT, the shape of bench/zoo/spec.json
// "observed"). ZOO_VARIANT=0|1 picks the danger, ZOO_REVERSE=1 runs the cases in the reverse order.
fn main() {
    let reverse = std::env::var("ZOO_REVERSE").as_deref() == Ok("1");
    let problems = zoo_exec::run_all(reverse);
    if let Ok(out) = std::env::var("ZOO_OUT") {
        if let Err(e) = std::fs::write(&out, zoo_mark::observation_json(reverse)) {
            eprintln!("cannot write {out}: {e}");
            std::process::exit(2);
        }
    }
    for p in &problems {
        eprintln!("{p}");
    }
    if !problems.is_empty() {
        std::process::exit(1);
    }
}
