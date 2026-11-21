//! The cells of the Rust zoo: one file is one cell, with three cases (a vulnerable one, a fixed one, a decoy).
//! See bench/zoo/README.md. `register()` of a file hands its cases to zoo-mark; the table of the files is in
//! exec/src/cases.rs (not analysed). The modules are inline, so that this is the only file without a cell.
//! (`dyns` is the family `dyn` of the design: `dyn` is a keyword.)
//! @na(F/conv/exec-output-echo: Rust's `Command::output` has no model with an approximation yet; the cell is not written)
//! @na(F/conv/struct-field-shallow: the sinks that name a field of a structure (`parts`) are measured in Go first; the Rust cells are stage 6 of docs/DESIGN-field-sinks.md)
//! @na(F/conv/struct-field-deep: the sinks that name a field of a structure (`parts`) are measured in Go first; the Rust cells are stage 6 of docs/DESIGN-field-sinks.md)
//! @na(F/conv/struct-field-boxed: the sinks that name a field of a structure (`parts`) are measured in Go first; the Rust cells are stage 6 of docs/DESIGN-field-sinks.md)
//! @na(F/conv/struct-field-opaque: the sinks that name a field of a structure (`parts`) are measured in Go first; the Rust cells are stage 6 of docs/DESIGN-field-sinks.md)
//! @na(F/dyn/iface-assert-ptr-recv: Go's type assertion of `any` to an interface read through a pointer receiver; Rust has neither)
//! @na(F/dyn/sanitizer-own-impl: not written for Rust yet; the built-in model has no sanitizer on a trait method, and the engine reads an implementation of a sanitizer's trait the same way in every language)
pub mod call {
    pub mod depth3;
    pub mod direct;
    pub mod direct_in;
    pub mod multiret;
}
pub mod clos {
    pub mod capture_write_by_ref;
    pub mod inline;
    pub mod partial_callers;
    pub mod relay;
    pub mod returned;
    pub mod stored;
    pub mod to_lib;
    pub mod to_own_helper;
    pub mod two_callers;
}
pub mod cont {
    pub mod map_key_vs_value;
    pub mod slice;
    pub mod struct_whole_to_sink;
}
pub mod conv {
    pub mod buffer_writestring;
}
pub mod dyns {
    pub mod func_field;
    pub mod func_map;
    pub mod generics_ptr;
    pub mod global_iface;
    pub mod http_middleware_chain;
    pub mod iface_ptr_recv_read;
    pub mod iface_ptr_recv_write;
    pub mod iface_val_recv;
    pub mod method_value;
    pub mod method_value_iface;
    pub mod sink_behind_own_interface;
    pub mod sink_in_method;
    pub mod sink_in_method_iface;
    pub mod stdlib_writer_own;
    pub mod xtalk_n_by_m;
}
pub mod guard {
    pub mod one_branch;
    pub mod sanitize_before;
}
pub mod rust {
    pub mod arc_mutex;
    pub mod assoc_fn;
    pub mod async_await;
    pub mod cfg_feature;
    pub mod channel;
    pub mod closure_fn;
    pub mod closure_fnmut;
    pub mod closure_fnonce;
    pub mod deref;
    pub mod display_format;
    pub mod drop_;
    pub mod dyn_vs_mono;
    pub mod from_into;
    pub mod iterator_chain;
    pub mod macro_rules;
    pub mod mut_out_param;
    pub mod option_result_chain;
    pub mod ownership_move;
    pub mod static_lazylock;
}
pub mod state {
    pub mod global_chain;
    pub mod global_direct;
    pub mod global_field;
    pub mod global_struct_copy;
    pub mod handler_then_reader;
    pub mod receiver_field_across_entries;
    pub mod save_restore;
}
