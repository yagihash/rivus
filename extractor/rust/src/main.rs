#![feature(rustc_private)]
//! rivus-rust: the Rust extractor of rivus.
//!
//! `rivus-rust [--include-tests] [directory]` writes the neutral IR of the Rust crates of the
//! workspace in the directory (the current one by default) to the standard
//! output, as JSON Lines. It builds the workspace with `cargo check`, running
//! itself in place of rustc for the crates of the workspace: it compiles them
//! as usual, and writes the IR of what rustc made of them (MIR, through
//! rustc_public) on the way.

extern crate rustc_driver;
extern crate rustc_interface;
extern crate rustc_middle;
#[macro_use]
extern crate rustc_public;

mod irjson;
mod lower;
mod traitdef;

use rustc_public::CrateDef;
use serde_json::{json, Value};
use std::collections::{HashMap, HashSet};
use std::path::{Path, PathBuf};
use std::process::{Command, ExitCode};

/// The `+toolchain` that goes first in the arguments of rustc and cargo (the proxies of rustup pick the toolchain by it).
/// With RIVUS_RUST_DIRECT set, nothing: `rustc` and `cargo` on PATH are the ones of the pinned toolchain itself (the
/// sandbox of macOS runs them so, without rustup: there is nothing to pick, and no proxy has to be allowed).
fn plus() -> Vec<String> {
    if std::env::var_os("RIVUS_RUST_DIRECT").is_some() {
        Vec::new()
    } else {
        vec![format!("+{}", TOOLCHAIN)]
    }
}

/// The toolchain the extractor is built with and runs rustc with.
/// It is read from rust-toolchain.toml by build.rs: the nightly is written in that one place.
const TOOLCHAIN: &str = env!("RIVUS_TOOLCHAIN");

/// Whether rustc is asked to compile the build script of a package: cargo names its crate `build_script_<name>`
/// (`build_script_build` for build.rs, and the name of the file for `build = "codegen.rs"`).
fn is_build_script(args: &[String]) -> bool {
    args.windows(2).any(|w| w[0] == "--crate-name" && w[1].starts_with("build_script_"))
        || args.iter().any(|a| a == "build_script_build" || a == "build_script_main")
}

/// A directory that only this process can use: made exclusively (making it fails when the name exists, whoever
/// made it), with the mode 0700, under a name that another user cannot wait for, and removed when the value is
/// dropped, whichever way the extraction ends. The build of the project goes into it (CARGO_TARGET_DIR), and so
/// do the fragments of the IR: a directory that someone else made first would be where they replace the build
/// scripts that run, or write the IR (yagihash/rivus #44).
struct ScratchDir(PathBuf);

impl ScratchDir {
    fn create() -> std::io::Result<ScratchDir> {
        let base = std::env::temp_dir();
        for _ in 0..100 {
            let path = base.join(format!("rivus-rust-{}-{:016x}", std::process::id(), unpredictable()));
            match make_private_dir(&path) {
                Ok(()) => return Ok(ScratchDir(path)),
                Err(e) if e.kind() == std::io::ErrorKind::AlreadyExists => continue,
                Err(e) => return Err(e),
            }
        }
        Err(std::io::Error::new(std::io::ErrorKind::AlreadyExists, "every name that was tried is taken"))
    }

    fn path(&self) -> &Path {
        &self.0
    }
}

impl Drop for ScratchDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

/// Makes the directory, and fails when it exists (a symbolic link to somewhere included).
#[cfg(unix)]
fn make_private_dir(path: &Path) -> std::io::Result<()> {
    use std::os::unix::fs::DirBuilderExt;
    std::fs::DirBuilder::new().mode(0o700).create(path)
}

#[cfg(not(unix))]
fn make_private_dir(path: &Path) -> std::io::Result<()> {
    std::fs::create_dir(path)
}

/// Eight bytes that are not a function of the process number: from the system, or else from the clock.
fn unpredictable() -> u64 {
    use std::io::Read;
    let mut b = [0u8; 8];
    if std::fs::File::open("/dev/urandom").and_then(|mut f| f.read_exact(&mut b)).is_ok() {
        return u64::from_le_bytes(b);
    }
    let nanos = std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH).map(|d| d.as_nanos()).unwrap_or(0) as u64;
    nanos ^ ((&b as *const _ as usize) as u64).rotate_left(17)
}

/// The hash of the source of this extractor (internal/extractorhash), given to the build in RIVUS_EXTRACTOR_SRC_HASH (`rivus setup rust` does;
/// build.rs reads it); "unknown" for a build without it.
const SOURCE_HASH: &str = env!("RIVUS_EXTRACTOR_SRC_HASH");

fn main() -> ExitCode {
    // Asked on purpose, by rivus and the tests: the schema of the IR and the hash of the source this extractor was built from (ADR-126). It is
    // answered first and from constants, before the driver mode (`--version` is the driver's: cargo asks it for the compiler's, so it is not
    // used) and before anything of the arguments, cargo or the target is touched: RIVUS_RUST_DRIVER in the environment does not matter.
    if std::env::args().nth(1).as_deref() == Some("--rivus-version") {
        println!("rivus-rust {} source={}", irjson::SCHEMA_VERSION, SOURCE_HASH);
        return ExitCode::SUCCESS;
    }
    if std::env::var_os("RIVUS_RUST_DRIVER").is_some() {
        return driver();
    }
    cli()
}

// --- the driver: runs in place of rustc -------------------------------------

fn driver() -> ExitCode {
    // Cargo runs a wrapper as `wrapper rustc <arguments>`: the compiler is the
    // program the driver pretends to be.
    let mut args: Vec<String> = std::env::args().skip(1).collect();
    let probing = args.iter().any(|a| a == "-vV" || a == "--version" || a.starts_with("--print"));
    let skipped = is_build_script(&args);
    // The optimizations of MIR change what is read: with `opt-level` above 0 rustc
    // inlines (and the call that a rule names is gone). The level is fixed, whatever
    // profile the workspace builds with.
    if !probing {
        args.push("-Zmir-opt-level=1".to_string());
    }
    let out_dir = std::env::var("RIVUS_IR_DIR").unwrap_or_default();
    let root = std::env::var("RIVUS_ROOT").unwrap_or_default();
    let crate_name = args.iter().position(|a| a == "--crate-name").and_then(|i| args.get(i + 1)).cloned().unwrap_or_default();

    // A crate that is not a member of the workspace is a dependency: cargo
    // says which are members. A proc macro runs in the compiler, not in the
    // program: it is not read.
    let dep = std::env::var_os("CARGO_PRIMARY_PACKAGE").is_none();
    let proc_macro = args.windows(2).any(|w| w[0] == "--crate-type" && w[1] == "proc-macro");

    // a dependency that a model declares (`--body-replaced`, told by the command in RIVUS_BODY_REPLACED): the body is not read
    let replaced = dep && body_replaced_crate(&std::env::var("RIVUS_BODY_REPLACED").unwrap_or_default(), &crate_name);

    let write_failed = std::sync::atomic::AtomicBool::new(false);
    let result = run_with_tcx!(&args, |tcx| {
        if !probing && !skipped && !proc_macro && !out_dir.is_empty() {
            // a crate that is built to be loaded or linked by another program is called from outside
            let open_crate = args.windows(2).any(|w| w[0] == "--crate-type" && matches!(w[1].as_str(), "cdylib" | "staticlib" | "dylib"))
                || args.iter().any(|a| matches!(a.as_str(), "--crate-type=cdylib" | "--crate-type=staticlib" | "--crate-type=dylib"));
            install_panic_hook();
            let records = extract(tcx, &root, dep, open_crate, &crate_name, replaced);
            // a test build of a crate (`--test`) is told apart by its name: the merge prefers it to the library build
            let kind = if args.iter().any(|a| a == "--test") { "-test" } else { "" };
            let path = PathBuf::from(&out_dir).join(format!("{}{}-{}.jsonl", crate_name, kind, std::process::id()));
            let mut text = String::new();
            for r in records {
                text.push_str(&r.to_string());
                text.push('\n');
            }
            if let Err(e) = std::fs::write(&path, text) {
                // a crate that is not written is a crate that is gone from the program: not a success
                eprintln!("rivus-rust: cannot write {}: {}", path.display(), e);
                write_failed.store(true, std::sync::atomic::Ordering::SeqCst);
            }
        }
        ControlFlow::<(), ()>::Continue(())
    });
    if (result.is_err() || write_failed.load(std::sync::atomic::Ordering::SeqCst)) && !probing {
        return ExitCode::from(1);
    }
    ExitCode::SUCCESS
}

/// Whether the crate is one of the declared ones (the names are separated by a line break). Cargo reads `-` and `_` of a crate name
/// as the same (`actix-web` of Cargo.lock and a model, `actix_web` of rustc), so the names are compared that way.
fn body_replaced_crate(declared: &str, crate_name: &str) -> bool {
    let norm = |s: &str| s.replace('-', "_");
    !crate_name.is_empty() && declared.lines().any(|d| !d.is_empty() && norm(d) == norm(crate_name))
}

/// The records of the crate being compiled: its functions, and the texts.
fn extract(tcx: rustc_middle::ty::TyCtxt<'_>, root: &str, dep: bool, open_crate: bool, crate_name: &str, replaced: bool) -> Vec<Value> {
    let items: Vec<_> = rustc_public::all_local_items().into_iter().filter(|i| i.has_body()).collect();
    if replaced {
        // The model stands for this crate: no body is lowered (no MIR is built for it here). cargo has compiled it as it does for
        // any dependency; what is left out is the neutral IR of it, and the analysis of it. The functions are counted from the
        // items that have a body (closures included), so that the report says how many functions the crate declares.
        // The names go with it (`names`, which merge reads and removes): a call through a trait and a drop of a value in another
        // crate are linked to the impls of the whole program (link_across_crates), and the impls of this crate are among them
        // as they are when it is read (the analysis then finds no body for them, as when the filter leaves them out).
        let names: Vec<String> = items.iter().map(|i| i.name()).collect();
        return vec![json!({"kind": "body_not_read", "body_not_read": {"unit": crate_name, "functions": items.len(), "names": names}})];
    }
    let bodies: HashSet<String> = items.iter().map(|i| i.name()).collect();
    let mut cx = lower::Ctx::new(root.to_string(), bodies);
    // where the trait of a call through a trait is defined (traitdef.rs, the one place that asks the compiler's own API)
    cx.callee_trait = Box::new(move |def| traitdef::trait_path_of_callee(tcx, def));
    // what the crate holds that is not read, said at the end (a load error that names the function)
    let mut unread: Vec<(String, String)> = Vec::new();
    // the functions whose body could not be converted (a panic of the conversion that was caught): name, why
    let mut skipped: Vec<(String, String)> = Vec::new();
    // the promoted constants of the crate: the functions in them are used as values (`&helper`)
    for ldid in tcx.mir_keys(()) {
        let did = ldid.to_def_id();
        if !tcx.is_mir_available(did) {
            // the constants that this function holds and that name a function (`&helper`) are not read: a flow
            // through such a function value is lost. (what input does this: not known; no fixture)
            unread.push((rustc_public::rustc_internal::stable(did).name(), "the MIR of this function is not available: the functions that its constants name as values are not read".to_string()));
            continue;
        }
        let promoted = tcx.promoted_mir(did);
        if promoted.is_empty() {
            continue;
        }
        let owner = rustc_public::rustc_internal::stable(did).name();
        for (i, b) in promoted.iter_enumerated() {
            let fns = lower::fns_named_by(&rustc_public::rustc_internal::stable(b));
            if !fns.is_empty() {
                cx.promoted.insert((owner.clone(), i.as_u32()), fns);
            }
        }
    }
    for t in rustc_public::local_crate().trait_decls() {
        let private = cx.trait_is_private(&t.span());
        cx.traits.insert(t.name(), private);
    }
    for item in &items {
        let name = item.name();
        if name.contains("{closure#") {
            // a body that cannot be converted is skipped here without a word: the loop below meets the same function and says it
            if let Ok(Some(tys)) = guarded(|| {
                item.body().and_then(|b| {
                    if lower::is_closure_body(&b) {
                        Some(b.arg_locals().iter().skip(1).map(|l| lower::closure_key(&l.ty)).collect::<Vec<_>>())
                    } else {
                        None
                    }
                })
            }) {
                cx.closures.insert(name, tys);
            }
        }
    }
    let mut out = Vec::new();
    // main of a binary crate: where the program starts
    let entry = rustc_public::entry_fn().map(|e| e.name());
    // a test build has two items of the same name for each `#[test]` function: the function and the constant that
    // the test harness makes of it (and the closure in that constant, named after it). What the harness makes is
    // not code of the program: the function is
    let mut count: HashMap<String, usize> = HashMap::new();
    for item in &items {
        *count.entry(item.name()).or_default() += 1;
    }
    for item in &items {
        let name = item.name();
        // The conversion of a body to stable MIR can panic (`unimplemented!` of rustc_public: async closures are not lowered
        // yet): that function is skipped, and said (fn_skipped), and the others are read (docs/DESIGN-rust-extractor-panic.md)
        let f = match guarded(|| item.body().map(|body| lower::Lower::lower(&mut cx, name.clone(), &body, item.span()))) {
            Ok(Some(f)) => f,
            Ok(None) => {
                // an item that has a body and gives none: left out (what input does this: not known; no fixture)
                unread.push((item.name(), "the body of this function could not be read: it is left out".to_string()));
                continue;
            }
            Err(why) => {
                skipped.push((name.clone(), why));
                continue;
            }
        };
        let mut j = f.to_json();
        if count.get(&name).copied().unwrap_or(0) > 1 {
            let text = j.to_string();
            if text.contains("test::TestDescAndFn") || text.contains("test::assert_test_result") {
                continue;
            }
        }
        if !dep && entry.as_deref() == Some(name.as_str()) {
            j["entry"] = json!(true);
        }
        // an impl of a trait says where the trait is defined: an internal mark, which link_across_crates reads and removes
        if let Some(t) = traitdef::trait_path_of_impl(tcx, item) {
            j[IMPL_TRAIT_MARK] = json!(t);
        }
        // what can be called from outside the program as it is read (a symbol that another program links:
        // `#[no_mangle]`, `#[export_name]`, `extern "C"`, anything of a crate built as a library of C), and
        // what cannot be called from outside its crate (no `pub`): the first is where the program is open
        // to callers that are not read, the second is where the callers are all in the crate
        if open_crate || is_external_entry(item) {
            j["external_entry"] = json!(true);
        } else if cx.item_is_private(&item.span(), &name) {
            j["private"] = json!(true);
        }
        if dep {
            // the texts of a dependency are not wanted: the core keeps only
            // the functions the code reaches
            j["dep"] = json!(true);
        } else {
            let source = cx.expr_text(&item.span());
            cx.texts.func(name.clone(), source.clone(), source);
        }
        out.push(json!({"kind": "func", "func": j}));
    }
    if !dep {
        cx.texts.write(&mut out);
    }
    report_skipped(crate_name, &skipped);
    for (name, why) in &skipped {
        out.push(json!({"kind": "fn_skipped", "fn_skipped": {"unit": crate_name, "func": name, "dependency": dep, "why": why}}));
    }
    for (name, why) in unread {
        out.push(json!({"kind": "load_error", "load_error": {"package": name, "message": why, "unread": true, "func": name}}));
    }
    out
}

/// Whether the function is one that another program can call by its symbol: its symbol is not the one that
/// rustc makes up (`#[no_mangle]`, `#[export_name]`), or it is not a function of Rust (`extern "C"`).
fn is_external_entry(item: &rustc_public::CrateItem) -> bool {
    use rustc_public::mir::mono::Instance;
    use rustc_public::ty::{RigidTy, TyKind};
    if let TyKind::RigidTy(RigidTy::FnDef(def, _)) = item.ty().kind() {
        if !lower::has_rust_abi(def) {
            return true;
        }
    }
    match Instance::try_from(*item) {
        Ok(inst) => {
            let sym = inst.mangled_name();
            !(sym.starts_with("_ZN") || sym.starts_with("_R") || sym.starts_with("__ZN") || sym.starts_with("__R"))
        }
        Err(_) => false,
    }
}

// --- the command: builds the workspace with the driver in place of rustc ------

fn cli() -> ExitCode {
    // `--include-tests` also builds the tests, examples and benchmarks (what
    // `cargo check --all-targets` builds). Without it only the library and the
    // binaries are built: the code that is shipped, and nothing else, by
    // construction (a `#[cfg(test)]` item is not compiled at all).
    let mut include_tests = false;
    let mut deps = true;
    let mut cargo_flags: Vec<&str> = Vec::new();
    let mut dir = None;
    let mut body_replaced: Vec<String> = Vec::new();
    let mut args = std::env::args().skip(1);
    while let Some(a) = args.next() {
        match a.as_str() {
            "--include-tests" | "--tests" | "-tests" => include_tests = true,
            "--no-deps" => deps = false,
            // the crates that a model declares: their bodies are not read (the crate name, with `-` or `_`: cargo reads them alike)
            "--body-replaced" => match args.next() {
                Some(c) if !c.is_empty() => body_replaced.push(c),
                // default-ok: not a syntax kind: argument parsing
                _ => {
                    eprintln!("rivus-rust: --body-replaced needs the name of a crate");
                    return ExitCode::from(2);
                }
            },
            // what the isolated run asks (docs/rust-isolation.md): the lock file is not written, and nothing is fetched
            "--locked" => cargo_flags.push("--locked"),
            "--offline" => cargo_flags.push("--offline"),
            // default-ok: not a syntax kind: argument parsing
            _ => dir = Some(a),
        }
    }
    let dir = dir.unwrap_or_else(|| ".".into());
    let root = match std::fs::canonicalize(&dir) {
        Ok(p) => p,
        Err(e) => {
            eprintln!("rivus-rust: {}: {}", dir, e);
            return ExitCode::from(2);
        }
    };
    // A directory only this process can use (see ScratchDir); it is removed on every way out of this function.
    let scratch = match ScratchDir::create() {
        Ok(d) => d,
        Err(e) => {
            eprintln!("rivus-rust: cannot make a directory to work in: {}", e);
            return ExitCode::from(1);
        }
    };
    let tmp = scratch.path().to_path_buf();
    let frags = tmp.join("fragments");
    if let Err(e) = std::fs::create_dir_all(&frags) {
        eprintln!("rivus-rust: cannot make {}: {}", frags.display(), e);
        return ExitCode::from(1);
    }
    let exe = std::env::current_exe().expect("the path of the extractor");
    // The toolchain has to be there: a missing one is said as that, not as code that does not compile.
    let sysroot = match Command::new("rustc").args(plus().iter().map(String::as_str).chain(["--print", "sysroot"])).output() {
        Ok(o) if o.status.success() => String::from_utf8_lossy(&o.stdout).trim().to_string(),
        r => {
            let why = match r {
                Ok(o) => String::from_utf8_lossy(&o.stderr).trim().to_string(),
                Err(e) => format!("cannot run rustc: {}", e),
            };
            eprintln!("rivus-rust: the toolchain {} is needed ({}); install it:", TOOLCHAIN, why);
            eprintln!("  rustup toolchain install {} --profile minimal -c rustc-dev -c llvm-tools -c rust-src", TOOLCHAIN);
            return ExitCode::from(1);
        }
    };
    // the library path of the compiler goes in front of what the environment has, which stays
    let with_lib = |var: &str| -> String {
        let lib = format!("{}/lib", sysroot);
        match std::env::var(var) {
            Ok(old) if !old.is_empty() => format!("{}:{}", lib, old),
            // default-ok: not a syntax kind: the environment variable is not set
            _ => lib,
        }
    };

    let mut check = vec!["check", "--workspace", "--quiet"];
    if include_tests {
        check.push("--all-targets");
    }
    let extra_flags: Vec<String> = cargo_flags.iter().map(|f| f.to_string()).collect();
    check.extend(cargo_flags);
    // With the dependencies, the driver stands in for rustc for every crate (cargo
    // tells the members of the workspace apart); without, for the members only.
    let wrapper = if deps { "RUSTC_WRAPPER" } else { "RUSTC_WORKSPACE_WRAPPER" };
    // cargo's own messages go to the standard error as they are; they are also kept, to name the crate that did not compile
    let mut cmd = Command::new("cargo");
    cmd.args(plus())
        .args(&check)
        .current_dir(&root)
        .env("CARGO_TARGET_DIR", tmp.join("target"))
        .env(wrapper, &exe)
        .env("RIVUS_RUST_DRIVER", "1")
        .env("RIVUS_IR_DIR", &frags)
        .env("RIVUS_ROOT", &root)
        .env("RIVUS_BODY_REPLACED", body_replaced.join("\n"))
        .env("DYLD_FALLBACK_LIBRARY_PATH", with_lib("DYLD_FALLBACK_LIBRARY_PATH"))
        .env("LD_LIBRARY_PATH", with_lib("LD_LIBRARY_PATH"))
        .stderr(std::process::Stdio::piped());
    let status = run_tee_stderr(&mut cmd);
    match status {
        Ok((s, _)) if s.success() => {}
        Ok((s, errs)) => {
            let msg = match failed_crate(&errs) {
                Some(name) => {
                    let known = crate_versions(&root, &tmp, &name, &extra_flags);
                    compile_failure_message(&s.to_string(), &name, &known, deps)
                }
                None => format!("rivus-rust: cargo check failed ({}): the code has to compile", s),
            };
            eprintln!("{}", msg);
            return ExitCode::from(1);
        }
        Err(e) => {
            eprintln!("rivus-rust: cannot run cargo: {}", e);
            return ExitCode::from(1);
        }
    }
    // a fragments directory that cannot be listed is a program that is gone: the stream would be the header alone
    let files = match fragment_files(&frags) {
        Ok(f) => f,
        Err(e) => {
            eprintln!("rivus-rust: cannot read the extracted code in {}: {}", frags.display(), e);
            return ExitCode::from(1);
        }
    };
    merge(files);
    // this extractor marks what only its crate can call (`private`) and what another program can call (`external_entry`)
    println!("{}", json!({"kind": "caller_marks", "caller_marks": true}));
    if !include_tests {
        // What is not built: the tests, examples and benchmarks of the workspace, and the dependencies declared for
        // development only (cargo check does not compile them without --all-targets). The build scripts are not
        // extracted either (is_build_script), and the crates they need only run at build time.
        if let Some(v) = tests_skipped(&root, &tmp) {
            println!("{}", json!({"kind": "tests_skipped", "tests_skipped": v}));
        }
    }
    ExitCode::SUCCESS
}

thread_local! {
    /// Set while a conversion runs under `guarded`: the hook of panics keeps the message and says nothing.
    static GUARDED: std::cell::Cell<bool> = const { std::cell::Cell::new(false) };
    /// The message (first line) and the place of the last panic under `guarded`.
    static LAST_PANIC: std::cell::RefCell<String> = const { std::cell::RefCell::new(String::new()) };
}

/// Puts a hook of panics in (once): a panic under `guarded` is not written (its first line and place are kept for the
/// summary line and for fn_skipped; RIVUS_RUST_PANIC_VERBOSE=1 writes it as it is); a panic anywhere else goes to the hook
/// that was there.
fn install_panic_hook() {
    static ONCE: std::sync::Once = std::sync::Once::new();
    ONCE.call_once(|| {
        let prev = std::panic::take_hook();
        std::panic::set_hook(Box::new(move |info| {
            if GUARDED.with(|g| g.get()) {
                LAST_PANIC.with(|m| *m.borrow_mut() = panic_summary(info));
                if std::env::var_os("RIVUS_RUST_PANIC_VERBOSE").is_none() {
                    return;
                }
            }
            prev(info);
        }));
    });
}

fn panic_summary(info: &std::panic::PanicHookInfo<'_>) -> String {
    let payload = info.payload();
    let msg = payload.downcast_ref::<&str>().map(|s| s.to_string()).or_else(|| payload.downcast_ref::<String>().cloned()).unwrap_or_else(|| "a panic".to_string());
    let first = msg.lines().next().unwrap_or("").to_string();
    match info.location() {
        // the place in the source of the compiler, without the directory of the toolchain: `rustc_public/src/.../ty.rs:484`
        Some(l) => format!("{} (at {}:{})", first, l.file().rsplit("/compiler/").next().unwrap_or(l.file()), l.line()),
        None => first,
    }
}

/// Runs `f`; a panic in it is caught, and the first line of its message is the error. Only for what is read from rustc
/// after the queries are done (the conversion of a body to stable MIR and the lowering of it): a panic inside a query
/// leaves the query active, and the compiler aborts later (lower.rs, `resolve`). If it aborts, the crate fails as before.
fn guarded<T>(f: impl FnOnce() -> T) -> Result<T, String> {
    GUARDED.with(|g| g.set(true));
    let r = std::panic::catch_unwind(std::panic::AssertUnwindSafe(f));
    GUARDED.with(|g| g.set(false));
    r.map_err(|_| LAST_PANIC.with(|m| std::mem::take(&mut *m.borrow_mut())))
}

/// How many skipped functions of a crate are said one by one on the standard error; the rest is counted in one line.
const SKIPPED_SAID: usize = 5;

/// One line for each of the first functions that were skipped, and one for the rest.
fn report_skipped(crate_name: &str, skipped: &[(String, String)]) {
    for (name, why) in skipped.iter().take(SKIPPED_SAID) {
        eprintln!("rivus-rust: skipped {} ({}): {}", name, crate_name, why);
    }
    if skipped.len() > SKIPPED_SAID {
        eprintln!("rivus-rust: ... and {} more functions skipped in {}", skipped.len() - SKIPPED_SAID, crate_name);
    }
}

/// Runs the command (its standard error piped) and passes its standard error on to ours line by line; returns the status and
/// the text of the standard error.
fn run_tee_stderr(cmd: &mut Command) -> std::io::Result<(std::process::ExitStatus, String)> {
    use std::io::{BufRead, Write};
    let mut child = cmd.spawn()?;
    let mut all = String::new();
    if let Some(e) = child.stderr.take() {
        let mut r = std::io::BufReader::new(e);
        let mut line = Vec::new();
        loop {
            line.clear();
            match r.read_until(b'\n', &mut line) {
                Ok(0) | Err(_) => break,
                Ok(_) => {
                    let _ = std::io::stderr().write_all(&line);
                    all.push_str(&String::from_utf8_lossy(&line));
                }
            }
        }
    }
    Ok((child.wait()?, all))
}

/// The name of the crate that cargo says it could not compile: "error: could not compile `NAME` (lib)" (the first one).
fn failed_crate(stderr: &str) -> Option<String> {
    for l in stderr.lines() {
        if let Some(rest) = l.trim_start().strip_prefix("error: could not compile `") {
            if let Some(end) = rest.find('`') {
                return Some(rest[..end].to_string());
            }
        }
    }
    None
}

/// What `cargo metadata` knows of the packages with this name: (version, is a member of the workspace).
fn crate_versions(root: &Path, tmp: &Path, name: &str, flags: &[String]) -> Vec<(String, bool)> {
    let out = Command::new("cargo")
        .args(plus())
        .args(["metadata", "--format-version", "1"])
        .args(flags)
        .current_dir(root)
        .env("CARGO_TARGET_DIR", tmp.join("target"))
        .output();
    let Ok(out) = out else { return Vec::new() };
    if !out.status.success() {
        return Vec::new();
    }
    let Ok(meta) = serde_json::from_slice::<Value>(&out.stdout) else { return Vec::new() };
    let members: HashSet<&str> = meta["workspace_members"].as_array().into_iter().flatten().filter_map(|m| m.as_str()).collect();
    let mut found = Vec::new();
    for p in meta["packages"].as_array().into_iter().flatten() {
        if p["name"].as_str() == Some(name) {
            let version = p["version"].as_str().unwrap_or("").to_string();
            let member = p["id"].as_str().map_or(false, |id| members.contains(id));
            found.push((version, member));
        }
    }
    found
}

/// The last message when cargo check fails and the crate that did not compile is known: a crate of the workspace is the
/// code of the user; a dependency is not (it is the extractor, or the compiler under it, that stopped on it).
fn compile_failure_message(status: &str, name: &str, known: &[(String, bool)], deps: bool) -> String {
    if known.iter().any(|(_, member)| *member) {
        return format!("rivus-rust: cargo check failed ({}): the crate {} of the project does not compile; the code has to compile", status, name);
    }
    let versions: Vec<&str> = known.iter().map(|(v, _)| v.as_str()).collect();
    let what = if versions.is_empty() { name.to_string() } else { format!("{} {}", name, versions.join(", ")) };
    let hint = if deps { " The project can be read without its dependencies: -no-deps." } else { "" };
    format!(
        "rivus-rust: cargo check failed ({}): the dependency {} did not compile under the extractor. This is not a fault of the code of the project: the extractor (or the compiler it uses, {}) stops on that crate; the lines above say how.{}",
        status, what, TOOLCHAIN, hint
    )
}

/// The targets that `cargo check` leaves out without `--include-tests` (kind test, example, bench), and the
/// dependencies declared under `[dev-dependencies]` only, read from `cargo metadata --no-deps`. None when
/// cargo does not say.
fn tests_skipped(root: &Path, tmp: &Path) -> Option<serde_json::Value> {
    let out = Command::new("cargo")
        .args(plus())
        .args(["metadata", "--no-deps", "--format-version", "1"])
        .current_dir(root)
        .env("CARGO_TARGET_DIR", tmp.join("target"))
        .output()
        .ok()?;
    if !out.status.success() {
        return None;
    }
    let meta: serde_json::Value = serde_json::from_slice(&out.stdout).ok()?;
    let mut targets = 0;
    let mut dev: std::collections::BTreeSet<String> = Default::default();
    let mut prod: std::collections::BTreeSet<String> = Default::default();
    for p in meta["packages"].as_array()? {
        if let Some(a) = p["name"].as_str() {
            prod.insert(a.to_string());
        }
        for t in p["targets"].as_array().into_iter().flatten() {
            if t["kind"].as_array().into_iter().flatten().any(|k| matches!(k.as_str(), Some("test") | Some("example") | Some("bench"))) {
                targets += 1;
            }
        }
        for d in p["dependencies"].as_array().into_iter().flatten() {
            let name = d["name"].as_str().unwrap_or("").to_string();
            if d["kind"].as_str() == Some("dev") {
                dev.insert(name);
            } else {
                prod.insert(name);
            }
        }
    }
    let names: Vec<String> = dev.difference(&prod).cloned().collect();
    if targets == 0 && names.is_empty() {
        return None;
    }
    Some(json!({"files": 0, "targets": targets, "dev_dependencies": names.len(), "dev_dependency_names": names.iter().take(50).collect::<Vec<_>>()}))
}

/// The files the driver wrote, in name order. A directory that cannot be read, or an entry that cannot be
/// read, is an error: nothing of the program may go missing without a trace.
fn fragment_files(frags: &Path) -> std::io::Result<Vec<PathBuf>> {
    let mut files = Vec::new();
    for e in std::fs::read_dir(frags)? {
        files.push(e?.path());
    }
    files.sort();
    Ok(files)
}

/// The crate a fragment is of, and whether it is the test build of it (`<crate>-test-<pid>.jsonl`).
fn fragment_origin(path: &Path) -> (String, bool) {
    let stem = path.file_stem().map(|s| s.to_string_lossy().to_string()).unwrap_or_default();
    let name = match stem.rsplit_once('-') {
        Some((n, _pid)) => n.to_string(),
        None => stem,
    };
    match name.strip_suffix("-test") {
        Some(c) => (c.to_string(), true),
        None => (name, false),
    }
}

/// Writes the stream: the header, the functions (a function the workspace
/// compiled twice, as a library and as a test, once), and the rest.
fn merge(files: Vec<PathBuf>) {
    println!("{}", json!({"schema_version": irjson::SCHEMA_VERSION}));
    // One function per name. Two crates that have a function of the same name (the same crate in two
    // versions that do not agree, or the library and its tests) cannot both be the program: the choice is
    // by the content, not by the order the compiler processes (a process number in a file name), and it is said.
    let mut funcs: std::collections::BTreeMap<String, String> = std::collections::BTreeMap::new();
    // where each function came from: the crate, and whether it is a test build of it
    let mut origin: std::collections::HashMap<String, (String, bool)> = std::collections::HashMap::new();
    let mut conflicts: Vec<String> = Vec::new();
    let mut rest = Vec::new();
    // the functions of the crates that are not read (body_not_read): their names take part in the linking, and nothing else of them
    // (a crate can be built twice, with other features or for another target: the unit is one, and its functions are counted by name)
    let mut unread: std::collections::BTreeMap<String, std::collections::BTreeSet<String>> = std::collections::BTreeMap::new();
    for f in files {
        let from = fragment_origin(&f);
        let text = match std::fs::read_to_string(&f) {
            Ok(t) => t,
            Err(e) => {
                // a crate that is not read is a crate that is gone from the program: the core records it
                eprintln!("rivus-rust: cannot read {}: {}", f.display(), e);
                rest.push(json!({"kind": "load_error", "load_error": {"package": f.display().to_string(), "message": format!("the extracted code of a crate could not be read: {}", e)}}).to_string());
                continue;
            }
        };
        for line in text.lines() {
            let Ok(v) = serde_json::from_str::<Value>(line) else {
                // a line that is cut (the driver was killed while it wrote)
                rest.push(json!({"kind": "load_error", "load_error": {"package": f.display().to_string(), "message": "a line of the extracted code is broken (a driver that was stopped?)"}}).to_string());
                continue;
            };
            if v["kind"] == "func" {
                let id = v["func"]["id"].as_str().unwrap_or("").to_string();
                match funcs.get(&id) {
                    None => {
                        origin.insert(id.clone(), from.clone());
                        funcs.insert(id, line.to_string());
                    }
                    Some(old) if old == line => {}
                    Some(old) => {
                        let was = origin.get(&id).cloned().unwrap_or_default();
                        if was.0 == from.0 && was.1 != from.1 {
                            // the library and its tests are one crate, built twice: the test build has the code of
                            // the library and what the tests add (the closures and impls the calls may reach), so it stays
                            if from.1 {
                                origin.insert(id.clone(), from.clone());
                                funcs.insert(id, line.to_string());
                            }
                        } else {
                            // not the same function: the smaller line stays, whichever came first
                            conflicts.push(id.clone());
                            if line < old.as_str() {
                                origin.insert(id.clone(), from.clone());
                                funcs.insert(id, line.to_string());
                            }
                        }
                    }
                }
            } else if v["kind"] == "body_not_read" {
                let unit = v["body_not_read"]["unit"].as_str().unwrap_or("").to_string();
                let names = v["body_not_read"]["names"].as_array().into_iter().flatten().filter_map(|n| n.as_str().map(String::from));
                unread.entry(unit).or_default().extend(names);
            } else {
                rest.push(line.to_string());
            }
        }
    }
    for (unit, names) in &unread {
        rest.push(json!({"kind": "body_not_read", "body_not_read": {"unit": unit, "functions": names.len()}}).to_string());
    }
    rest.extend(link_across_crates(&mut funcs, unread.into_values().flatten().collect()));
    for l in funcs.values() {
        println!("{}", l);
    }
    // the first twenty are named; the total is said, so that the size of it is known
    let total = conflicts.len();
    for id in conflicts.into_iter().take(20) {
        println!(
            "{}",
            json!({"kind": "load_error", "load_error": {"package": id, "message": format!("two crates have a function of this name and they differ (a crate in two versions?): one of them is in the program ({} such functions in all)", total)}})
        );
    }
    if total > 20 {
        println!(
            "{}",
            json!({"kind": "load_error", "load_error": {"package": "(crates in two versions)", "message": format!("{} functions have a name that two crates share and they differ; only the first 20 are named", total)}})
        );
    }
    // The records are alone (the reader takes each by its kind): they are written in the order of their text, not in the
    // order of the fragments. A package with a library and a binary of one name has two fragments of that name, and which
    // comes first is by the number of the process (and `999` is after `1000`), so the stream would differ from run to run.
    rest.sort();
    for l in rest {
        println!("{}", l);
    }
}

/// The internal marks of the trait that a function or a call is of, by where the trait is defined (`traitdef.rs`). The
/// one on a function (an impl of a trait) is at the top of its record; the one on a call is in its `callee`, next to
/// `by_trait`. `link_across_crates` reads and removes them: they are not fields of the stream that is written (the
/// reader of the core stops on a field that it does not know).
const IMPL_TRAIT_MARK: &str = "impl_trait_def";
const CALL_TRAIT_MARK: &str = "by_trait_def";

/// Where the first string field `key` is in a line of JSON: the range of the field (with one comma next to it) and its value.
/// The line is not parsed: the field is searched by its text, which cannot be inside a string of the line (a quote in a
/// string is written `\"`, so the text `"key":"` does not occur in one).
fn find_mark(line: &str, key: &str) -> Option<(usize, usize, String)> {
    let pat = format!("\"{}\":\"", key);
    let start = line.find(&pat)?;
    let value_start = start + pat.len() - 1;
    let bytes = line.as_bytes();
    let mut i = value_start + 1;
    while i < bytes.len() && bytes[i] != b'"' {
        i += if bytes[i] == b'\\' { 2 } else { 1 };
    }
    if i >= bytes.len() {
        return None;
    }
    let value: String = serde_json::from_str(&line[value_start..=i]).ok()?;
    let end = i + 1;
    Some(if line[..start].ends_with(',') {
        (start - 1, end, value)
    } else if line[end..].starts_with(',') {
        (start, end + 1, value)
    } else {
        (start, end, value)
    })
}

/// The line without any field `key`.
fn strip_marks(line: &str, key: &str) -> String {
    let mut out = line.to_string();
    while let Some((a, b, _)) = find_mark(&out, key) {
        out.replace_range(a..b, "");
    }
    out
}

/// Whether the impl `name` (`<T as path::Trait>::method`) is of the trait that the call names. When the call says where its trait is
/// defined and the impl says where its trait is, those are compared (the same crate and path, whatever name each of them uses for
/// the trait); when either could not be told, the names are compared as before (`same_trait`).
fn impl_is_of_trait(called_def: Option<&str>, impl_def: Option<&str>, name: &str, suffix: &str, trait_path: &str) -> bool {
    if let (Some(c), Some(i)) = (called_def, impl_def) {
        return c == i;
    }
    lower::impl_of_trait(name, suffix).map_or(false, |t| lower::same_trait(t, trait_path))
}

/// The calls through a trait (`dyn Trait`, a type parameter) and the drops of values of another crate, which
/// one crate cannot resolve: it knows the impls of its own functions only. Here every crate is in, so the
/// candidates are the impls of the whole program, and the marks the lowering left are removed.
fn link_across_crates(funcs: &mut std::collections::BTreeMap<String, String>, unread: Vec<String>) -> Vec<String> {
    let mut unlinked: Vec<String> = Vec::new();
    let mut by_method: std::collections::HashMap<String, Vec<String>> = std::collections::HashMap::new();
    let mut drops: std::collections::HashMap<String, Vec<String>> = std::collections::HashMap::new();
    const DROP_TAIL: &str = " as std::ops::Drop>::drop";
    // the names of the functions that are read and of the ones that are not (in one order: the first impl of a drop is the same
    // with and without the crate being read)
    // where the trait of each impl is defined (the mark on the function, removed here from every line, one that cannot be read too).
    // The line is searched by its text, as `by_trait` and `drop_of` are: the lines of the functions are not parsed for this
    let mut impl_def: std::collections::HashMap<String, String> = std::collections::HashMap::new();
    for (id, line) in funcs.iter_mut() {
        if let Some((a, b, v)) = find_mark(line, IMPL_TRAIT_MARK) {
            line.replace_range(a..b, "");
            impl_def.insert(id.clone(), v);
        }
    }
    let mut all: Vec<&String> = funcs.keys().chain(unread.iter()).collect();
    all.sort();
    all.dedup();
    for id in all {
        if !id.starts_with('<') {
            continue;
        }
        if let Some((_, method)) = id.rsplit_once(">::") {
            by_method.entry(method.to_string()).or_default().push(id.clone());
        }
        if id.ends_with(DROP_TAIL) {
            let inner = &id[1..id.len() - DROP_TAIL.len()];
            drops.entry(inner.split('<').next().unwrap_or(inner).to_string()).or_default().push(id.clone());
        }
    }
    let ids: HashSet<String> = funcs.keys().cloned().chain(unread).collect();
    for (id, line) in funcs.iter_mut() {
        if !line.contains("\"by_trait\"") && !line.contains("\"drop_of\"") {
            continue;
        }
        let Ok(mut v) = serde_json::from_str::<Value>(line) else {
            // the line stays as it is (reported as unread), without the marks of its calls: they are not fields of the stream
            *line = strip_marks(line, CALL_TRAIT_MARK);
            unlinked.push(json!({"kind": "load_error", "load_error": {"package": id, "message": "the calls through a trait and the drops of this function could not be linked across the crates: its stream line is not readable", "unread": true, "func": id}}).to_string());
            continue;
        };
        let Some(blocks) = v["func"]["blocks"].as_array_mut() else { continue };
        for b in blocks {
            let Some(instrs) = b["instrs"].as_array_mut() else { continue };
            for in_ in instrs {
                let Some(callee) = in_.get_mut("callee").and_then(|c| c.as_object_mut()) else { continue };
                let called_def = callee.remove(CALL_TRAIT_MARK).and_then(|d| d.as_str().map(String::from));
                if callee.remove("by_trait").is_some() {
                    let sym = callee.get("symbol").and_then(|s| s.as_str()).unwrap_or("").to_string();
                    let mut cands: Vec<String> = callee.get("candidates").and_then(|c| c.as_array()).map(|a| a.iter().filter_map(|x| x.as_str().map(String::from)).collect()).unwrap_or_default();
                    if let Some((trait_path, method)) = sym.rsplit_once("::") {
                        let suffix = format!(">::{}", method);
                        for n in by_method.get(method).map(|v| v.as_slice()).unwrap_or(&[]) {
                            if impl_is_of_trait(called_def.as_deref(), impl_def.get(n).map(|s| s.as_str()), n, &suffix, trait_path) {
                                cands.push(n.clone());
                            }
                        }
                    }
                    // the default body of the method, which an impl that does not override it uses
                    if ids.contains(&sym) {
                        cands.push(sym.clone());
                    }
                    cands.sort();
                    cands.dedup();
                    callee.insert("candidates".into(), json!(cands));
                }
                if let Some(want) = callee.remove("drop_of") {
                    let found = want.as_str().and_then(|w| drops.get(w)).cloned().unwrap_or_default();
                    match found.first() {
                        Some(name) => {
                            callee.clear();
                            callee.insert("kind".into(), json!("static"));
                            callee.insert("func".into(), json!(name));
                            callee.insert("symbol".into(), json!("std::ops::Drop::drop"));
                        }
                        None => {
                            // no impl: the call is not made (a value that nothing runs a destructor for)
                            let pos = in_["pos"].clone();
                            let dst = in_["dsts"].clone();
                            *in_ = json!({"op": "const", "dsts": dst, "const": {"kind": "nil"}, "pos": pos});
                        }
                    }
                }
            }
        }
        *line = v.to_string();
    }
    unlinked
}

#[cfg(test)]
mod tests {
    use super::*;

    fn args(v: &[&str]) -> Vec<String> {
        v.iter().map(|s| s.to_string()).collect()
    }

    // a build script of another name was read as shipped code, and its main became an entry (yagihash/rivus #47)
    // the library and its test build are one crate, told apart by the name of the fragment (yagihash/rivus #68)
    #[test]
    fn a_fragment_says_its_crate_and_whether_it_is_a_test_build() {
        assert_eq!(fragment_origin(Path::new("/t/frags/app-1234.jsonl")), ("app".to_string(), false));
        assert_eq!(fragment_origin(Path::new("/t/frags/app-test-1234.jsonl")), ("app".to_string(), true));
        assert_eq!(fragment_origin(Path::new("/t/frags/my_crate-test-7.jsonl")), ("my_crate".to_string(), true));
    }

    #[test]
    fn the_scratch_directory_is_private_exclusive_and_removed() {
        let (a, b) = (ScratchDir::create().unwrap(), ScratchDir::create().unwrap());
        assert_ne!(a.path(), b.path());
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            assert_eq!(std::fs::metadata(a.path()).unwrap().permissions().mode() & 0o777, 0o700);
        }
        // a directory that exists is not taken over, and neither is a link
        assert_eq!(make_private_dir(a.path()).unwrap_err().kind(), std::io::ErrorKind::AlreadyExists);
        #[cfg(unix)]
        {
            let link = std::env::temp_dir().join(format!("rivus-rust-test-link-{}", unpredictable()));
            std::os::unix::fs::symlink(a.path(), &link).unwrap();
            assert!(make_private_dir(&link).is_err());
            std::fs::remove_file(&link).unwrap();
        }
        let path = a.path().to_path_buf();
        drop(a);
        assert!(!path.exists(), "the directory is left behind");
    }

    // a directory that cannot be listed gave a stream of the header alone and the exit code 0 (yagihash/rivus #46)
    #[test]
    fn a_fragment_directory_that_cannot_be_read_is_an_error() {
        let dir = ScratchDir::create().unwrap();
        std::fs::write(dir.path().join("b"), "").unwrap();
        std::fs::write(dir.path().join("a"), "").unwrap();
        let names: Vec<_> = fragment_files(dir.path()).unwrap().iter().map(|p| p.file_name().unwrap().to_string_lossy().to_string()).collect();
        assert_eq!(names, ["a", "b"]);
        assert!(fragment_files(&dir.path().join("gone")).is_err());
    }

    #[test]
    fn a_build_script_of_any_name_is_one() {
        assert!(is_build_script(&args(&["rustc", "--crate-name", "build_script_build", "build.rs"])));
        assert!(is_build_script(&args(&["rustc", "--crate-name", "build_script_codegen", "codegen.rs"])));
        assert!(!is_build_script(&args(&["rustc", "--crate-name", "mylib", "src/lib.rs"])));
    }

    #[test]
    fn a_panic_under_guarded_is_caught_with_its_first_line_and_place_and_the_next_call_goes_on() {
        install_panic_hook();
        let e = guarded(|| -> u32 { unimplemented!("FIXME(x): lower\nmore") }).unwrap_err();
        assert!(e.starts_with("not implemented: FIXME(x): lower (at "), "{e}");
        assert!(!e.contains("more"), "{e}");
        assert_eq!(guarded(|| 7).unwrap(), 7);
    }

    const GOOGLE_AUTH: &str = "thread 'rustc' panicked at /rustc-dev/x/ty.rs:484:17:\nnot implemented: FIXME(async_closures): Lower these to SMIR\nerror: could not compile `google-cloud-auth` (lib)\n\nCaused by:\n  process didn't exit successfully: `rivus-rust rustc --crate-name google_cloud_auth` (exit status: 101)\n";

    #[test]
    fn a_declared_crate_is_found_with_either_spelling_of_its_name() {
        assert!(body_replaced_crate("actix-web", "actix_web"));
        assert!(body_replaced_crate("serde\nactix_web", "actix_web"));
        assert!(body_replaced_crate("actix_web", "actix-web"));
        assert!(!body_replaced_crate("actix-web", "actix_web_codegen"));
        assert!(!body_replaced_crate("", ""));
        assert!(!body_replaced_crate("actix", "actix_web"));
    }

    #[test]
    fn the_failed_crate_is_the_one_cargo_names() {
        assert_eq!(failed_crate(GOOGLE_AUTH).as_deref(), Some("google-cloud-auth"));
        assert_eq!(failed_crate("error[E0433]: cannot find x\nerror: could not compile `f` (lib) due to 1 previous error\n").as_deref(), Some("f"));
        assert_eq!(failed_crate("warning: unused\n"), None);
    }

    #[test]
    fn a_dependency_that_does_not_compile_is_said_as_not_the_code_of_the_project() {
        let m = compile_failure_message("exit status: 101", "google-cloud-auth", &[("1.19.0".into(), false)], true);
        assert!(m.contains("the dependency google-cloud-auth 1.19.0"), "{m}");
        assert!(m.contains("not a fault of the code of the project"), "{m}");
        assert!(m.contains("-no-deps"), "{m}");
        assert!(!m.contains("the code has to compile"), "{m}");
        // already without the dependencies: no advice to leave them out
        assert!(!compile_failure_message("exit status: 101", "x", &[("1.0.0".into(), false)], false).contains("-no-deps"));
    }

    #[test]
    fn a_crate_of_the_project_is_still_the_code_that_has_to_compile() {
        let m = compile_failure_message("exit status: 101", "mine", &[("0.1.0".into(), true)], true);
        assert!(m.contains("the code has to compile"), "{m}");
        assert!(!m.contains("-no-deps"), "{m}");
    }

    // the marks of the trait that a function or a call is of (traitdef.rs): read from the text of a line, and removed from it
    #[test]
    fn a_mark_is_found_in_the_text_of_a_line_and_removed_with_one_comma() {
        let line = r#"{"func":{"blocks":[],"id":"<A as t::Tr>::m","impl_trait_def":"zoo_web::service::Tr","pos":{}}}"#;
        let (a, b, v) = find_mark(line, IMPL_TRAIT_MARK).unwrap();
        assert_eq!(v, "zoo_web::service::Tr");
        let mut out = line.to_string();
        out.replace_range(a..b, "");
        assert_eq!(out, r#"{"func":{"blocks":[],"id":"<A as t::Tr>::m","pos":{}}}"#);
        // the last field of the object, and the first
        assert_eq!(strip_marks(r#"{"a":1,"impl_trait_def":"x::T"}"#, IMPL_TRAIT_MARK), r#"{"a":1}"#);
        assert_eq!(strip_marks(r#"{"impl_trait_def":"x::T","a":1}"#, IMPL_TRAIT_MARK), r#"{"a":1}"#);
        // every one of a line (a call each), and none in a string that has the text in it
        assert_eq!(strip_marks(r#"{"c":[{"by_trait_def":"a::T","k":1},{"k":2,"by_trait_def":"b::T"}]}"#, CALL_TRAIT_MARK), r#"{"c":[{"k":1},{"k":2}]}"#);
        let quoted = r#"{"s":"\"impl_trait_def\":\"x\""}"#;
        assert!(find_mark(quoted, IMPL_TRAIT_MARK).is_none(), "{quoted}");
        assert!(find_mark(r#"{"a":1}"#, IMPL_TRAIT_MARK).is_none());
        // a line that is cut inside the value has no mark
        assert!(find_mark(r#"{"impl_trait_def":"x::T"#, IMPL_TRAIT_MARK).is_none());
    }

    // the marks decide when both are there; when either is not, the names are compared as before (same_trait)
    #[test]
    fn a_trait_is_told_by_where_it_is_defined_and_by_its_name_when_that_is_not_known() {
        let (suffix, called) = (">::register", "zoo_web::service::Tr");
        let imp = "<h as zoo_web::dev::Tr>::register";
        // a re-export in the middle: the names differ, the definition is one
        assert!(!lower::same_trait("zoo_web::dev::Tr", called));
        assert!(impl_is_of_trait(Some("zoo_web::service::Tr"), Some("zoo_web::service::Tr"), imp, suffix, called));
        // the same name, another definition
        assert!(!impl_is_of_trait(Some("zoo_web::service::Tr"), Some("app::other::Tr"), "<l as zoo_web::service::Tr>::register", suffix, called));
        // a mark that could not be made on either side: the names (the root re-export of ADR-058 and the traits told apart by their path)
        assert!(!impl_is_of_trait(None, Some("zoo_web::service::Tr"), imp, suffix, called));
        assert!(!impl_is_of_trait(Some("zoo_web::service::Tr"), None, imp, suffix, called));
        assert!(impl_is_of_trait(None, None, "<S as parseable::handlers::http::modal::ParseableServer>::init", ">::init", "parseable::ParseableServer"));
        assert!(impl_is_of_trait(Some("parseable::ParseableServer"), None, "<S as parseable::handlers::http::modal::ParseableServer>::init", ">::init", "parseable::ParseableServer"));
        assert!(!impl_is_of_trait(None, None, "<S as app::http::Server>::run", ">::run", "app::Handler::run"));
    }

    // link_across_crates: the impls whose mark is the call's get into the candidates, a lookalike does not, one with no mark is taken by
    // its name, and no mark is left in any line
    #[test]
    fn linking_compares_the_marks_and_leaves_none() {
        let call = |sym: &str, mark: Option<&str>| {
            let mut callee = json!({"kind": "dynamic", "symbol": sym, "candidates": [], "by_trait": true});
            if let Some(m) = mark {
                callee["by_trait_def"] = json!(m);
            }
            json!({"func": {"id": "caller", "blocks": [{"instrs": [{"op": "call", "callee": callee}]}]}}).to_string()
        };
        let imp = |id: &str, mark: Option<&str>| {
            let mut f = json!({"id": id, "blocks": []});
            if let Some(m) = mark {
                f["impl_trait_def"] = json!(m);
            }
            json!({"func": f}).to_string()
        };
        let mut funcs = std::collections::BTreeMap::new();
        funcs.insert("caller".to_string(), call("zoo_web::service::Tr::register", Some("zoo_web::service::Tr")));
        funcs.insert("<h as zoo_web::dev::Tr>::register".to_string(), imp("<h as zoo_web::dev::Tr>::register", Some("zoo_web::service::Tr")));
        funcs.insert("<l as app::other::Tr>::register".to_string(), imp("<l as app::other::Tr>::register", Some("app::other::Tr")));
        // no mark: the names are compared, and the names of these two are not the call's
        funcs.insert("<n as zoo_web::dev::Tr>::register".to_string(), imp("<n as zoo_web::dev::Tr>::register", None));
        funcs.insert("<m as zoo_web::service::Tr>::register".to_string(), imp("<m as zoo_web::service::Tr>::register", None));
        // a call with no mark: the names (a root re-export is the same trait)
        funcs.insert("caller2".to_string(), call("zoo_web::Tr::register", None).replace("caller", "caller2"));
        funcs.insert("<r as zoo_web::service::Tr>::register".to_string(), imp("<r as zoo_web::service::Tr>::register", Some("zoo_web::service::Tr")));
        // a line that cannot be read keeps nothing of the marks
        funcs.insert("<broken as t::T>::x".to_string(), r#"{"func":{"by_trait":true,"callee":{"by_trait_def":"a::T"},"id":"<broken as t::T>::x","impl_trait_def":"a::T""#.to_string());
        let unlinked = link_across_crates(&mut funcs, vec![]);
        assert_eq!(unlinked.len(), 1);
        for (id, line) in &funcs {
            assert!(!line.contains("impl_trait_def") && !line.contains("by_trait_def"), "{id}: {line}");
        }
        let cands = |id: &str| -> Vec<String> {
            let v: Value = serde_json::from_str(&funcs[id]).unwrap();
            v["func"]["blocks"][0]["instrs"][0]["callee"]["candidates"].as_array().unwrap().iter().map(|c| c.as_str().unwrap().to_string()).collect()
        };
        assert_eq!(cands("caller"), vec!["<h as zoo_web::dev::Tr>::register", "<m as zoo_web::service::Tr>::register", "<r as zoo_web::service::Tr>::register"]);
        // (m has no mark and its name is the call's: by name; n has no mark and another name: left out, as before)
        // the root path (zoo_web::Tr) is the same trait as the other two paths, by name
        assert_eq!(cands("caller2").len(), 4);
        assert!(!cands("caller2").iter().any(|c| c.contains("app::other")));
    }
}
