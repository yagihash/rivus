pub mod across;
pub mod callbacks;
pub mod capture;
pub mod errors;
pub mod fnself;
pub mod fnvalues;
pub mod helpers;
pub mod iters;
pub mod modelled;
pub mod narrow;
pub mod nesting;
pub mod parseable;
pub mod pass_closure;
pub mod spelling;
pub mod structs;

// The source and the sink of the benchmark.
pub fn source() -> String {
    String::from("x")
}

pub fn sink(_s: &str) {}
