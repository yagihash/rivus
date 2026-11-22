// Runs the probe (RIVUS_PROBE_BIN, with the arguments RIVUS_PROBE_ARGS) and puts what it says in HOSTILE_REPORT, which src/lib.rs hands to the IR.
// It does nothing when it is not under the sandbox: a program that can list the real home directory is outside it.
use std::process::Command;

fn main() {
    // under the sandbox the home directory of the user is neither listable nor visible (the profile denies it): HOME here is a work directory,
    // so the real one is looked for by its owner's name in /Users
    if std::fs::read_dir("/Users").is_ok() {
        panic!("the build script that is hostile on purpose is for the sandbox only: /Users can be listed here, so this is not it");
    }
    let bin = std::env::var("RIVUS_PROBE_BIN").expect("RIVUS_PROBE_BIN");
    let args: Vec<String> = std::env::var("RIVUS_PROBE_ARGS").unwrap_or_default().split('\u{1}').filter(|s| !s.is_empty()).map(String::from).collect();
    let out = Command::new(&bin).args(&args).output();
    let report = match out {
        Ok(o) => String::from_utf8_lossy(&o.stdout).replace('\n', ";"),
        Err(e) => format!("PROBE_NOT_RUN {}", e),
    };
    println!("cargo:rustc-env=HOSTILE_REPORT={}", report);
    println!("cargo:rerun-if-changed=build.rs");
}
