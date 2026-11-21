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
    c::clos::partial_callers::register();
    c::clos::relay::register();
    c::clos::returned::register();
    c::clos::stored::register();
    c::clos::to_lib::register();
    c::clos::capture_write_by_ref::register();
    c::clos::to_own_helper::register();
    c::clos::two_callers::register();
    c::cont::map_key_vs_value::register();
    c::cont::slice::register();
    c::cont::struct_whole_to_sink::register();
    c::conv::buffer_writestring::register();
    c::dyns::func_field::register();
    c::dyns::func_map::register();
    c::dyns::generics_ptr::register();
    c::dyns::global_iface::register();
    c::dyns::http_middleware_chain::register();
    c::dyns::iface_ptr_recv_read::register();
    c::dyns::iface_ptr_recv_write::register();
    c::dyns::iface_val_recv::register();
    c::dyns::method_value::register();
    c::dyns::method_value_iface::register();
    c::dyns::sink_behind_own_interface::register();
    c::dyns::sink_in_method::register();
    c::dyns::sink_in_method_iface::register();
    c::dyns::stdlib_writer_own::register();
    c::dyns::xtalk_n_by_m::register();
    c::guard::one_branch::register();
    c::guard::sanitize_before::register();
    c::rust::arc_mutex::register();
    c::rust::assoc_fn::register();
    c::rust::async_await::register();
    c::rust::cfg_feature::register();
    c::rust::channel::register();
    c::rust::closure_fn::register();
    c::rust::closure_fnmut::register();
    c::rust::closure_fnonce::register();
    c::rust::deref::register();
    c::rust::display_format::register();
    c::rust::drop_::register();
    c::rust::dyn_vs_mono::register();
    c::rust::from_into::register();
    c::rust::iterator_chain::register();
    c::rust::macro_rules::register();
    c::rust::mut_out_param::register();
    c::rust::option_result_chain::register();
    c::rust::ownership_move::register();
    c::rust::static_lazylock::register();
    c::state::global_chain::register();
    c::state::global_direct::register();
    c::state::global_field::register();
    c::state::global_struct_copy::register();
    c::state::handler_then_reader::register();
    c::state::receiver_field_across_entries::register();
    c::state::save_restore::register();
}
