//! The cells of the package google_cloud_storage of the Rust zoo: one file is one cell (three cases). The modules are listed here;
//! the harness of the test runs the `register()` of every file of this directory.
pub mod storage_read_object;
pub mod storage_write_object;
pub mod storage_open_object;
pub mod control_get_object;
pub mod control_delete_object;
pub mod control_move_object;
pub mod control_rewrite_object;
pub mod control_list_objects;
pub mod signed_url_for_object;
pub mod signed_url_for_bucket;
