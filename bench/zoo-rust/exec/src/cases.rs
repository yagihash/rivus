// The cells register their cases in `register()`; the harness calls them all. Every file of cells/src is listed
// here (cmd/rivus TestZooTruthRust checks it). rivus does not read this file: a table of function values would make
// every case a stored function value of the analysis.
use zoo_cells as c;

pub fn register_all() {
    c::call::depth3::register();
    c::call::direct::register();
    c::call::direct_in::register();
    c::call::multiret::register();
    c::clos::inline::register();
    c::clos::returned::register();
    c::clos::to_own_helper::register();
    c::cont::map_key_vs_value::register();
    c::cont::slice::register();
    c::cont::struct_whole_to_sink::register();
    c::conv::buffer_writestring::register();
    c::dyns::generics_ptr::register();
    c::dyns::global_iface::register();
    c::dyns::http_middleware_chain::register();
    c::dyns::iface_ptr_recv_read::register();
    c::dyns::iface_ptr_recv_write::register();
    c::dyns::iface_val_recv::register();
    c::dyns::method_value::register();
    c::dyns::method_value_iface::register();
    c::dyns::sink_in_method::register();
    c::dyns::sink_in_method_iface::register();
    c::dyns::stdlib_writer_own::register();
    c::dyns::xtalk_n_by_m::register();
    c::guard::one_branch::register();
    c::guard::sanitize_before::register();
    c::state::global_chain::register();
    c::state::global_direct::register();
    c::state::handler_then_reader::register();
    c::state::receiver_field_across_entries::register();
    c::state::save_restore::register();
}
