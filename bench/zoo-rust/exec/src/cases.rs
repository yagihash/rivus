// The cells register their cases in `register()`; the harness calls them all. Every file of cells/src is listed
// here (cmd/rivus TestZooTruthRust checks it). rivus does not read this file: a table of function values would make
// every case a stored function value of the analysis.
use zoo_cells as c;

pub fn register_all() {
    c::call::direct::register();
}
