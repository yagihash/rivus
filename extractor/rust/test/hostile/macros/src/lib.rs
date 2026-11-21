// A proc macro runs in the compiler, with the rights of whoever builds: it tries the same as build.rs does.
use proc_macro::TokenStream;
use std::net::{TcpStream, ToSocketAddrs};
use std::time::Duration;

#[proc_macro]
pub fn attack(_: TokenStream) -> TokenStream {
    // this crate is built only in the image of the isolation: anywhere else it does nothing
    if !std::path::Path::new("/etc/rivus-isolated").exists() {
        return TokenStream::new();
    }
    let dir = std::env::var("CARGO_MANIFEST_DIR").unwrap_or_default();
    let canary = std::fs::read_to_string(format!("{}/../canary.txt", dir)).unwrap_or_default();
    let host_file = canary.lines().next().unwrap_or("");
    let read = std::fs::read_to_string(host_file).is_ok();
    let net = "1.1.1.1:53".to_socket_addrs().ok().and_then(|mut a| a.next()).map(|a| TcpStream::connect_timeout(&a, Duration::from_secs(3)).is_ok()).unwrap_or(false);
    let write = std::fs::write("/macro-wrote-here", "x").is_ok();
    for (name, escaped) in [("macro_read_host_file", read), ("macro_network", net), ("macro_write_root", write)] {
        eprintln!("HOSTILE {}={}", name, if escaped { "ESCAPED" } else { "BLOCKED" });
    }
    TokenStream::new()
}
