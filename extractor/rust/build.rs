// The extractor links against the compiler's own libraries (rustc_driver): it
// has to find them where the toolchain keeps them, wherever that is.
fn main() {
    let rustc = std::env::var("RUSTC").unwrap_or_else(|_| "rustc".into());
    let out = std::process::Command::new(rustc).args(["--print", "sysroot"]).output().expect("rustc --print sysroot");
    let sysroot = String::from_utf8(out.stdout).unwrap();
    let lib = format!("{}/lib", sysroot.trim());
    println!("cargo:rustc-link-arg=-Wl,-rpath,{}", lib);
    // the nightly the extractor is pinned to is written once, in rust-toolchain.toml
    let toml = std::fs::read_to_string("rust-toolchain.toml").expect("rust-toolchain.toml");
    let channel = toml
        .lines()
        .find_map(|l| l.trim().strip_prefix("channel").and_then(|r| r.trim().strip_prefix('=')).map(|r| r.trim().trim_matches('"').to_string()))
        .expect("a channel in rust-toolchain.toml");
    println!("cargo:rustc-env=RIVUS_TOOLCHAIN={}", channel);
    println!("cargo:rerun-if-changed=rust-toolchain.toml");
    println!("cargo:rerun-if-changed=build.rs");
}
