// Every case runs once (a case that panics, or that records no call of a sink, fails), in both orders.
// cmd/rivus TestZooExecRust runs the same thing three times in different processes, with coverage.
#[test]
fn every_case_runs() {
    let problems = zoo_exec::run_all(false);
    assert!(problems.is_empty(), "{problems:?}");
    assert!(zoo_mark::case_ids().len() >= 3);
}
