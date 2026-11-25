//! The cells of the Rust zoo: one file is one cell, with three cases (a vulnerable one, a fixed one, a decoy).
//! See bench/zoo/README.md. `register()` of a file hands its cases to zoo-mark; the table of the files is in
//! exec/src/cases.rs (not analysed). The modules are inline, so that this is the only file without a cell.
//! (`dyns` is the family `dyn` of the design: `dyn` is a keyword.)
//! Two marks for a cell of the Go zoo that Rust has not (docs/DESIGN-zoo.md, 4.2.5): `@na` is a form that Rust does not have at all,
//! `@todo` is a form that Rust can have and the cell is not written yet. The parity marks line of `TestZooRust` counts them apart.
//! @na(F/clos/range-func: Go's range over a function (Go 1.23: `for v := range seq`, the iterator calls yield); Rust has no loop that calls a closure for the body: an iterator is driven by `next`, and a `for` over it is a call of `next`)
//! @na(F/call/regexp-replace: Go's regexp replacement with a function and with a literal replacement; the regex crate is a dependency, and the Rust zoo has none)
//! @na(F/dyn/iface-assert-ptr-recv: Go's type assertion of `any` to an interface read through a pointer receiver; Rust has neither)
//! @todo(F/dyn/iface-write-passed: PARITY: Go's pointer held in an `io.Writer` interface value that a function writes to (ADR-090); a Rust `&mut dyn Write` passed to a function and read back from the owner can be written)
//! @todo(F/cont/ptr-commaok-assert: PARITY: Go's pointer taken out of an interface by a comma-ok assertion and written through; Rust can write through `&mut T` from `Any::downcast_mut::<T>()` on a `&mut dyn Any` and read the owner back; the cell is not written yet)
//! @todo(F/cont/ptr-commaok-map: PARITY: Go's pointer taken out of a map by a comma-ok index and written through; Rust can write through the `&mut V` of `if let Some(p) = map.get_mut(..)` and read the map back; the cell is not written yet)
//! @todo(F/cont/ptr-range-map: PARITY: Go's pointer that is the value of a range over a map and written through; Rust can write through the `&mut V` of `for p in map.values_mut()` and read the map back; the cell is not written yet)
//! @todo(F/cont/ptr-range-slice: PARITY: Go's pointer that is the value of a range over a slice and written through; Rust can write through the `&mut T` of `for p in v.iter_mut()` and read the vector back; the cell is not written yet)
//! @todo(F/conv/char-decode-rune: PARITY: Go's buffer type that reads the text with `utf8.DecodeRuneInString` in a loop, appends `byte(r)` to a `[]byte` field and gives the text back by slicing; Rust can read `char_indices()` or `chars()`, push `c as u8` to a `Vec<u8>` field and slice it back; the cell is not written yet)
//! @todo(F/conv/char-range-append: PARITY: Go's range loop over the text that appends `byte(r)` to a `[]byte` converted back whole; Rust can loop over `chars()` and push `c as u8` to a `Vec<u8>`, then `String::from_utf8`; the cell is not written yet)
//! @todo(F/clos/to-lib-where: PARITY: a closure given to a function of the standard library that tests the elements of a collection (Go: slices.IndexFunc, ContainsFunc, DeleteFunc); Rust has `Vec::retain`, `Iterator::position`, `any` and the like; the cell is not written yet)
pub mod call {
    pub mod depth3;
    pub mod direct;
    pub mod direct_in;
    pub mod multiret;
}
pub mod clos {
    pub mod capture_write_by_ref;
    pub mod inline;
    pub mod local_fn_from_closure;
    pub mod local_recursion;
    pub mod partial_callers;
    pub mod relay;
    pub mod returned;
    pub mod src_ref;
    pub mod stored;
    pub mod to_later;
    pub mod to_lib;
    pub mod to_lib_call;
    pub mod to_lib_map;
    pub mod to_lib_ref;
    pub mod to_nomodel;
    pub mod to_nomodel_ref;
    pub mod to_own_helper;
    pub mod to_sort;
    pub mod two_callers;
}
pub mod cont {
    pub mod map_key_vs_value;
    pub mod ptr_index_slice;
    pub mod ptr_type_switch;
    pub mod ptr_value_copy;
    pub mod slice;
    pub mod struct_whole_to_sink;
}
pub mod conv {
    pub mod buffer_writestring;
    pub mod builtin_minmax;
    pub mod char_append;
    pub mod char_base64;
    pub mod char_case;
    pub mod char_hex;
    pub mod char_index;
    pub mod char_lexer;
    pub mod char_range_write;
    pub mod char_replace;
    pub mod char_reverse;
    pub mod char_rune_slice;
    pub mod unsafe_string;
    pub mod exec_output_echo;
    pub mod struct_field_boxed;
    pub mod struct_field_deep;
    pub mod struct_field_opaque;
    pub mod struct_field_shallow;
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
    pub mod sink_as_value;
    pub mod sink_as_value_map;
    pub mod sink_as_value_field;
    pub mod sink_behind_own_interface;
    pub mod sink_in_method;
    pub mod sanitizer_own_impl;
    pub mod sink_in_method_iface;
    pub mod through_dependency;
    pub mod fprint_writer_own;
    pub mod stdlib_writer_own;
    pub mod stdlib_writer_own_static;
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
    pub mod vec_index_write;
}
pub mod san {
    pub mod other_kind;
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
