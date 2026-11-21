//! The harness of the Rust zoo (docs/DESIGN-zoo.md, 3.6): registers the cases of the cells and runs them.
//! cmd/rivus TestZooExecRust runs `zoo-observe` three times (variant 0, variant 1, variant 0 in the reverse order,
//! in different processes) with -C instrument-coverage; `cargo test` runs every case once (a case that panics, or
//! that records no call of a sink, fails) and the self-check of zoo-mark (tests/spec.rs).
use std::panic::{catch_unwind, AssertUnwindSafe};

pub mod cases;

/// Runs every case (in the reverse order when asked), each under `catch_unwind`. Returns the problems.
pub fn run_all(reverse: bool) -> Vec<String> {
    cases::register_all();
    let mut ids = zoo_mark::case_ids();
    if reverse {
        ids.reverse();
    }
    let mut problems = Vec::new();
    for id in ids {
        let before = zoo_mark::hit_count();
        let r = catch_unwind(AssertUnwindSafe(|| zoo_mark::run_case(&id)));
        if r.is_err() {
            problems.push(format!("{id}: the case panicked"));
        } else if zoo_mark::hit_count() <= before {
            problems.push(format!("{id}: the case has recorded no call of a sink"));
        }
    }
    problems
}
