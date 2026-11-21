//! The cells of the Rust zoo: one file is one cell, with three cases (a vulnerable one, a fixed one, a decoy).
//! See bench/zoo/README.md. `register()` of a file hands its cases to zoo-mark; the table of the files is in
//! exec/src/cases.rs (not analysed). The modules are inline, so that this is the only file without a cell.
//! (`dyns` is the family `dyn` of the design: `dyn` is a keyword.)
pub mod call {
    pub mod depth3;
    pub mod direct;
    pub mod direct_in;
    pub mod multiret;
}
pub mod clos {
    pub mod inline;
    pub mod returned;
    pub mod to_own_helper;
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
    pub mod generics_ptr;
    pub mod global_iface;
    pub mod http_middleware_chain;
    pub mod iface_ptr_recv_read;
    pub mod iface_ptr_recv_write;
    pub mod iface_val_recv;
    pub mod method_value;
    pub mod method_value_iface;
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
}
pub mod state {
    pub mod global_chain;
    pub mod global_direct;
    pub mod handler_then_reader;
    pub mod receiver_field_across_entries;
    pub mod save_restore;
}
