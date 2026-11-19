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

use rustc_public::CrateDef;
use serde_json::{json, Value};
use std::collections::HashSet;
use std::path::{Path, PathBuf};
use std::process::{Command, ExitCode};

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

fn main() -> ExitCode {
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

    let write_failed = std::sync::atomic::AtomicBool::new(false);
    let result = run!(&args, || {
        if !probing && !skipped && !proc_macro && !out_dir.is_empty() {
            let records = extract(&root, dep);
            let path = PathBuf::from(&out_dir).join(format!("{}-{}.jsonl", crate_name, std::process::id()));
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

/// The records of the crate being compiled: its functions, and the texts.
fn extract(root: &str, dep: bool) -> Vec<Value> {
    let items: Vec<_> = rustc_public::all_local_items().into_iter().filter(|i| i.has_body()).collect();
    let bodies: HashSet<String> = items.iter().map(|i| i.name()).collect();
    let mut cx = lower::Ctx::new(root.to_string(), bodies);
    for item in &items {
        let name = item.name();
        if name.contains("{closure#") {
            if let Some(b) = item.body() {
                if lower::is_closure_body(&b) {
                    let tys = b.arg_locals().iter().skip(1).map(|l| lower::closure_key(&l.ty)).collect();
                    cx.closures.insert(name, tys);
                }
            }
        }
    }
    let mut out = Vec::new();
    // main of a binary crate: where the program starts
    let entry = rustc_public::entry_fn().map(|e| e.name());
    for item in &items {
        let Some(body) = item.body() else { continue };
        let name = item.name();
        let f = lower::Lower::lower(&mut cx, name.clone(), &body, item.span());
        let mut j = f.to_json();
        if !dep && entry.as_deref() == Some(name.as_str()) {
            j["entry"] = json!(true);
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
    out
}

// --- the command: builds the workspace with the driver in place of rustc ------

fn cli() -> ExitCode {
    // `--include-tests` also builds the tests, examples and benchmarks (what
    // `cargo check --all-targets` builds). Without it only the library and the
    // binaries are built: the code that is shipped, and nothing else, by
    // construction (a `#[cfg(test)]` item is not compiled at all).
    let mut include_tests = false;
    let mut deps = true;
    let mut dir = None;
    for a in std::env::args().skip(1) {
        match a.as_str() {
            "--include-tests" | "--tests" | "-tests" => include_tests = true,
            "--no-deps" => deps = false,
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
    let sysroot = match Command::new("rustc").args([&format!("+{}", TOOLCHAIN), "--print", "sysroot"]).output() {
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
            _ => lib,
        }
    };

    let mut check = vec!["check", "--workspace", "--quiet"];
    if include_tests {
        check.push("--all-targets");
    }
    // With the dependencies, the driver stands in for rustc for every crate (cargo
    // tells the members of the workspace apart); without, for the members only.
    let wrapper = if deps { "RUSTC_WRAPPER" } else { "RUSTC_WORKSPACE_WRAPPER" };
    let status = Command::new("cargo")
        .arg(format!("+{}", TOOLCHAIN))
        .args(&check)
        .current_dir(&root)
        .env("CARGO_TARGET_DIR", tmp.join("target"))
        .env(wrapper, &exe)
        .env("RIVUS_RUST_DRIVER", "1")
        .env("RIVUS_IR_DIR", &frags)
        .env("RIVUS_ROOT", &root)
        .env("DYLD_FALLBACK_LIBRARY_PATH", with_lib("DYLD_FALLBACK_LIBRARY_PATH"))
        .env("LD_LIBRARY_PATH", with_lib("LD_LIBRARY_PATH"))
        .status();
    match status {
        Ok(s) if s.success() => {}
        Ok(s) => {
            eprintln!("rivus-rust: cargo check failed ({}): the code has to compile", s);
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
    ExitCode::SUCCESS
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

/// Writes the stream: the header, the functions (a function the workspace
/// compiled twice, as a library and as a test, once), and the rest.
fn merge(files: Vec<PathBuf>) {
    println!("{}", json!({"schema_version": irjson::SCHEMA_VERSION}));
    // One function per name. Two crates that have a function of the same name (the same crate in two
    // versions that do not agree, or the library and its tests) cannot both be the program: the choice is
    // by the content, not by the order the compiler processes (a process number in a file name), and it is said.
    let mut funcs: std::collections::BTreeMap<String, String> = std::collections::BTreeMap::new();
    let mut conflicts: Vec<String> = Vec::new();
    let mut rest = Vec::new();
    for f in files {
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
                        funcs.insert(id, line.to_string());
                    }
                    Some(old) if old == line => {}
                    Some(old) => {
                        // not the same function: the smaller line stays, whichever came first
                        if conflicts.len() < 20 {
                            conflicts.push(id.clone());
                        }
                        if line < old.as_str() {
                            funcs.insert(id, line.to_string());
                        }
                    }
                }
            } else {
                rest.push(line.to_string());
            }
        }
    }
    link_across_crates(&mut funcs);
    for l in funcs.values() {
        println!("{}", l);
    }
    for id in conflicts {
        println!(
            "{}",
            json!({"kind": "load_error", "load_error": {"package": id, "message": "two crates have a function of this name and they differ (a crate in two versions?): one of them is in the program"}})
        );
    }
    for l in rest {
        println!("{}", l);
    }
}

/// The calls through a trait (`dyn Trait`, a type parameter) and the drops of values of another crate, which
/// one crate cannot resolve: it knows the impls of its own functions only. Here every crate is in, so the
/// candidates are the impls of the whole program, and the marks the lowering left are removed.
fn link_across_crates(funcs: &mut std::collections::BTreeMap<String, String>) {
    let mut by_method: std::collections::HashMap<String, Vec<String>> = std::collections::HashMap::new();
    let mut drops: std::collections::HashMap<String, Vec<String>> = std::collections::HashMap::new();
    const DROP_TAIL: &str = " as std::ops::Drop>::drop";
    for id in funcs.keys() {
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
    let ids: HashSet<String> = funcs.keys().cloned().collect();
    for line in funcs.values_mut() {
        if !line.contains("\"by_trait\"") && !line.contains("\"drop_of\"") {
            continue;
        }
        let Ok(mut v) = serde_json::from_str::<Value>(line) else { continue };
        let Some(blocks) = v["func"]["blocks"].as_array_mut() else { continue };
        for b in blocks {
            let Some(instrs) = b["instrs"].as_array_mut() else { continue };
            for in_ in instrs {
                let Some(callee) = in_.get_mut("callee").and_then(|c| c.as_object_mut()) else { continue };
                if callee.remove("by_trait").is_some() {
                    let sym = callee.get("symbol").and_then(|s| s.as_str()).unwrap_or("").to_string();
                    let mut cands: Vec<String> = callee.get("candidates").and_then(|c| c.as_array()).map(|a| a.iter().filter_map(|x| x.as_str().map(String::from)).collect()).unwrap_or_default();
                    if let Some((trait_path, method)) = sym.rsplit_once("::") {
                        let trait_name = trait_path.rsplit("::").next().unwrap_or(trait_path);
                        let suffix = format!(">::{}", method);
                        for n in by_method.get(method).map(|v| v.as_slice()).unwrap_or(&[]) {
                            if lower::impl_of_trait(n, &suffix) == Some(trait_name) {
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
}

#[cfg(test)]
mod tests {
    use super::*;

    fn args(v: &[&str]) -> Vec<String> {
        v.iter().map(|s| s.to_string()).collect()
    }

    // a build script of another name was read as shipped code, and its main became an entry (yagihash/rivus #47)
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
}
