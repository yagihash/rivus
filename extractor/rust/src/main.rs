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
const TOOLCHAIN: &str = "nightly-2026-11-15";

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
    let args: Vec<String> = std::env::args().skip(1).collect();
    let probing = args.iter().any(|a| a == "-vV" || a == "--version" || a.starts_with("--print"));
    let skipped = args.iter().any(|a| a == "build_script_build" || a == "build_script_main");
    let out_dir = std::env::var("RIVUS_IR_DIR").unwrap_or_default();
    let root = std::env::var("RIVUS_ROOT").unwrap_or_default();
    let crate_name = args.iter().position(|a| a == "--crate-name").and_then(|i| args.get(i + 1)).cloned().unwrap_or_default();

    // A crate that is not a member of the workspace is a dependency: cargo
    // says which are members. A proc macro runs in the compiler, not in the
    // program: it is not read.
    let dep = std::env::var_os("CARGO_PRIMARY_PACKAGE").is_none();
    let proc_macro = args.windows(2).any(|w| w[0] == "--crate-type" && w[1] == "proc-macro");

    let result = run!(&args, || {
        if !probing && !skipped && !proc_macro && !out_dir.is_empty() {
            let records = extract(&root, dep);
            let path = PathBuf::from(&out_dir).join(format!("{}-{}.jsonl", crate_name, std::process::id()));
            let mut text = String::new();
            for r in records {
                text.push_str(&r.to_string());
                text.push('\n');
            }
            let _ = std::fs::write(path, text);
        }
        ControlFlow::<(), ()>::Continue(())
    });
    if result.is_err() && !probing {
        return ExitCode::from(1);
    }
    ExitCode::SUCCESS
}

/// The records of the crate being compiled: its functions, and the texts.
fn extract(root: &str, dep: bool) -> Vec<Value> {
    let items: Vec<_> = rustc_public::all_local_items().into_iter().filter(|i| i.has_body()).collect();
    let bodies: HashSet<String> = items.iter().map(|i| i.name()).collect();
    let mut cx = lower::Ctx::new(root.to_string(), bodies);
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
    let tmp = std::env::temp_dir().join(format!("rivus-rust-{}", std::process::id()));
    let frags = tmp.join("fragments");
    let _ = std::fs::create_dir_all(&frags);
    let exe = std::env::current_exe().expect("the path of the extractor");
    let sysroot = Command::new("rustc")
        .args([&format!("+{}", TOOLCHAIN), "--print", "sysroot"])
        .output()
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .unwrap_or_default();

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
        .env("DYLD_FALLBACK_LIBRARY_PATH", format!("{}/lib", sysroot))
        .env("LD_LIBRARY_PATH", format!("{}/lib", sysroot))
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
    merge(&frags);
    let _ = std::fs::remove_dir_all(&tmp);
    ExitCode::SUCCESS
}

/// Writes the stream: the header, the functions (a function the workspace
/// compiled twice, as a library and as a test, once), and the rest.
fn merge(frags: &Path) {
    let mut files: Vec<PathBuf> = std::fs::read_dir(frags).map(|d| d.filter_map(|e| e.ok().map(|e| e.path())).collect()).unwrap_or_default();
    files.sort();
    println!("{}", json!({"schema_version": irjson::SCHEMA_VERSION}));
    let mut seen = HashSet::new();
    let mut rest = Vec::new();
    for f in files {
        let text = std::fs::read_to_string(&f).unwrap_or_default();
        for line in text.lines() {
            let Ok(v) = serde_json::from_str::<Value>(line) else { continue };
            if v["kind"] == "func" {
                let id = v["func"]["id"].as_str().unwrap_or("").to_string();
                if seen.insert(id) {
                    println!("{}", line);
                }
            } else {
                rest.push(line.to_string());
            }
        }
    }
    for l in rest {
        println!("{}", l);
    }
}
