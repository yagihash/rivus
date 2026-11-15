// The extractor links against the compiler's own libraries (rustc_driver): it
// has to find them where the toolchain keeps them, wherever that is.
fn main() {
    let rustc = std::env::var("RUSTC").unwrap_or_else(|_| "rustc".into());
    let out = std::process::Command::new(rustc).args(["--print", "sysroot"]).output().expect("rustc --print sysroot");
    let sysroot = String::from_utf8(out.stdout).unwrap();
    let lib = format!("{}/lib", sysroot.trim());
    println!("cargo:rustc-link-arg=-Wl,-rpath,{}", lib);
    println!("cargo:rerun-if-changed=build.rs");
}
