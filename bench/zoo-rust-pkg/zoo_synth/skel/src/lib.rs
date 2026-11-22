//! Stands for an external package of the zoo (docs/DESIGN-package-models.md, 6.8): a skeleton in a crate of its own, so that the
//! cells of bench/zoo-rust-pkg/zoo_synth call a function of another crate. It is not a package of anyone.

/// Returns what it was given.
pub fn echo(s: &str) -> String {
    s.to_string()
}

/// Stands for a function with a sink row: it records its argument (the place of the record is the call in the cell:
/// `track_caller` hands the place of the cell to zoo-mark).
#[track_caller]
pub fn run(cmd: &str) {
    zoo_mark::sink_cmd(cmd);
}
