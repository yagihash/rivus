//! The identity of a trait, by where it is defined: the name of its crate and its path in the crate.
//!
//! The name that `rustc_public` gives a trait is the path rustc prints, and rustc prints a trait by a path that the code
//! uses (a `pub use` in the middle of a crate gives it a second name: `actix_web::service::HttpServiceFactory` is
//! `actix_web::dev::HttpServiceFactory`). Comparing those names (`lower::same_trait`) takes the two for two traits. The
//! definition has one path: the crate that defines the trait, and the path of the trait in it, which a `pub use` does
//! not change. A call through a trait and the impls of the trait carry that path as an internal mark, and
//! `link_across_crates` (main.rs) compares the marks, and removes them. A mark that could not be made is not there, and the
//! comparison of the names stays for it (`same_trait`), so a call or an impl without a mark is no worse off than before.
//!
//! This is the only place of the extractor that asks the compiler's own (internal) API, `TyCtxt`, a question that
//! `rustc_public` does not answer: which trait an `impl` is of, which trait a method belongs to, and where a trait is
//! defined (what `rustc_public` gives for a trait is the printed path). What this depends on is
//! `rustc_internal::internal` (the conversion to the compiler's `DefId`), `TyCtxt::{opt_parent, def_kind, impl_trait_ref,
//! crate_name, def_path}` and `DefKind`. When the pinned nightly (rust-toolchain.toml, ADR-013) is moved, this file is
//! what may not build any more (a name that moved); the tests of this file and the cells of bench/zoo-rust-app say
//! if it builds and gives something else. The string is the crate name and the path, never a hash of the definition
//! (`def_path_hash`: it changes with the compiler), and never a number of `rustc_public::DefId` (it is not the same in two
//! processes, and the crates of a program are read by one process each).

extern crate rustc_hir;

use rustc_middle::ty::TyCtxt;
use rustc_public::rustc_internal::internal;
use rustc_public::ty::FnDef;
use rustc_public::{CrateDef, CrateItem};

/// The path of a definition: `zoo_web::service::HttpServiceFactory`.
fn def_path(tcx: TyCtxt<'_>, did: rustc_hir::def_id::DefId) -> String {
    format!("{}::{}", tcx.crate_name(did.krate), tcx.def_path(did).to_string_no_crate_verbose().trim_start_matches("::"))
}

/// The trait that the impl function `item` is of (`impl Trait for T { fn item }`), if it is one.
pub(crate) fn trait_path_of_impl(tcx: TyCtxt<'_>, item: &CrateItem) -> Option<String> {
    let did = internal(tcx, item);
    let parent = tcx.opt_parent(did)?;
    if !matches!(tcx.def_kind(parent), rustc_hir::def::DefKind::Impl { of_trait: true }) {
        return None;
    }
    Some(def_path(tcx, tcx.impl_trait_ref(parent).skip_binder().def_id))
}

/// The trait that the method `def` belongs to (the callee of a call through a trait), if it belongs to one.
pub(crate) fn trait_path_of_callee(tcx: TyCtxt<'_>, def: FnDef) -> Option<String> {
    let did = internal(tcx, &def.def_id());
    let parent = tcx.opt_parent(did)?;
    if !matches!(tcx.def_kind(parent), rustc_hir::def::DefKind::Trait) {
        return None;
    }
    Some(def_path(tcx, parent))
}
