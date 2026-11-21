//! What the cases of the zoo call: the sources, the sinks and the sanitizers (docs/DESIGN-zoo.md, 3.2 and 3.5).
//!
//! The analysis reads `decl.rs`: the declarations, with empty bodies, so that no state of the recording is seen
//! (every case would write to one global). rivus gets the meaning of each function from rules.yaml. The recording
//! is `exec.rs` (feature `exec`); the marker and the judgement of "reached" are in bench/zoo/spec.json.
mod types;
pub use types::*;

#[cfg(not(feature = "exec"))]
mod decl;
#[cfg(not(feature = "exec"))]
pub use decl::*;

#[cfg(feature = "exec")]
mod exec;
#[cfg(feature = "exec")]
pub use exec::*;
