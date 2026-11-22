//! Stands for an external package of the zoo (docs/DESIGN-package-models.md, 6.8): a skeleton in a crate of its own, so that the
//! cells of bench/zoo-rust-pkg/zoo_synth call a function of another crate. It is not a package of anyone.

/// Returns what it was given.
pub fn echo(s: &str) -> String {
    s.to_string()
}
