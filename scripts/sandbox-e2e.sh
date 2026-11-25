#!/usr/bin/env bash
# The end-to-end check of the sandbox on this machine (macOS: Seatbelt, Linux: Landlock and seccomp; docs/rust-isolation.md).
#
#   RIVUS_RUST_DATA_DIR=<dir where `rivus setup rust` put the extractor> scripts/sandbox-e2e.sh <path of rivus>
#
# It makes a crate whose build script is hostile on purpose (it tries to read a secret, to write outside, to connect to a listener, to read an
# environment variable that the caller set; every one of them makes the build fail with ESCAPED) and that depends on a crate with a proc macro and a
# build script (serde), reads it with `rivus analyze -lang rust` (the fetch stage with the network, then the extraction without it), and checks:
#   - the run succeeded (so the build script ran, was held in, and could run the C compiler),
#   - summary.isolation.mode is "sandbox",
#   - nothing was written outside.
# As a control, the same build script is run directly on this machine, where it must fail with ESCAPED: otherwise "nothing escaped" shows nothing.
# Everything that the hostile build script touches is a file or a listener that this script made, in a directory that it removes at the end.
set -euo pipefail

RIVUS=${1:?usage: sandbox-e2e.sh <path of rivus>}
: "${RIVUS_RUST_DATA_DIR:?RIVUS_RUST_DATA_DIR must name the directory that rivus setup rust prepared}"
RIVUS=$(cd "$(dirname "$RIVUS")" && pwd)/$(basename "$RIVUS")

ROOT=$(mktemp -d)
LISTENER_PID=""
cleanup() {
  [ -n "$LISTENER_PID" ] && kill "$LISTENER_PID" 2>/dev/null || true
  rm -rf "$ROOT"
}
trap cleanup EXIT

OUTSIDE=$ROOT/outside
CANARY=$ROOT/canary
mkdir -p "$OUTSIDE"
echo "FAKE-SECRET" >"$CANARY"

# a listener on the loopback that the build script must not reach
PORT=$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')
python3 -m http.server "$PORT" --bind 127.0.0.1 --directory "$ROOT" >/dev/null 2>&1 &
LISTENER_PID=$!
for _ in $(seq 1 50); do
  python3 -c "import socket; socket.create_connection(('127.0.0.1', $PORT), 1).close()" 2>/dev/null && break
  sleep 0.1
done

write_build_rs() { # $1 = directory of the crate
  cat >"$1/build.rs" <<EOF
use std::{fs, net::TcpStream, process::Command};
fn main() {
    let cc = Command::new("cc").arg("--version").output().map(|o| o.status.success()).unwrap_or(false);
    let mut escaped: Vec<&str> = vec![];
    if fs::read_to_string("$CANARY").is_ok() { escaped.push("read-secret"); }
    if fs::write("$OUTSIDE/marker", "x").is_ok() { escaped.push("write-outside"); }
    if TcpStream::connect("127.0.0.1:$PORT").is_ok() { escaped.push("network"); }
    if std::env::var("SBXE2E_SECRET").is_ok() { escaped.push("environment"); }
    if !escaped.is_empty() { panic!("ESCAPED: {}", escaped.join(",")); }
    if !cc { panic!("the C compiler could not be run by the build script"); }
}
EOF
}

# the control: no dependency, the same build script
CTRL=$ROOT/control
mkdir -p "$CTRL/src"
printf '[package]\nname = "control"\nversion = "0.1.0"\nedition = "2021"\nbuild = "build.rs"\n' >"$CTRL/Cargo.toml"
printf 'version = 4\n\n[[package]]\nname = "control"\nversion = "0.1.0"\n' >"$CTRL/Cargo.lock"
echo 'fn main() {}' >"$CTRL/src/main.rs"
write_build_rs "$CTRL"

# the project that is read under the sandbox: the same build script, and a proc macro and build scripts of dependencies
PROJ=$ROOT/proj
mkdir -p "$PROJ/src"
cat >"$PROJ/Cargo.toml" <<'EOF'
[package]
name = "sbxe2e"
version = "0.1.0"
edition = "2021"
build = "build.rs"

[dependencies]
serde = { version = "1", features = ["derive"] }
shlex = "1"
EOF
cat >"$PROJ/src/main.rs" <<'EOF'
use serde::Deserialize;

#[derive(Deserialize)]
struct Req {
    cmd: String,
}

fn main() {
    let req: Req = Req { cmd: std::env::args().nth(1).unwrap_or_default() };
    let parts = shlex::split(&req.cmd).unwrap_or_default();
    if let Some((prog, args)) = parts.split_first() {
        let _ = std::process::Command::new(prog).args(args).status();
    }
}
EOF
write_build_rs "$PROJ"
# the lock file: cargo generate-lockfile runs no code of the project
(cd "$PROJ" && cargo generate-lockfile >/dev/null 2>&1)

export SBXE2E_SECRET=hunter2
echo "== control: the build script, run directly on this machine, must escape"
if cargo check --quiet --manifest-path "$CTRL/Cargo.toml" --target-dir "$ROOT/control-target" >"$ROOT/control.log" 2>&1; then
  echo "FAIL: the control did not escape: the checks of the build script show nothing" >&2
  cat "$ROOT/control.log" >&2
  exit 1
fi
if ! grep -q "ESCAPED" "$ROOT/control.log"; then
  echo "FAIL: the control failed for another reason" >&2
  cat "$ROOT/control.log" >&2
  exit 1
fi
echo "ok: $(grep -o 'ESCAPED: [a-z,-]*' "$ROOT/control.log" | head -1)"
rm -f "$OUTSIDE/marker"

echo "== under the sandbox"
if ! "$RIVUS" analyze -lang rust -C "$PROJ" >"$ROOT/out.json" 2>"$ROOT/err.txt"; then
  echo "FAIL: rivus analyze -lang rust failed" >&2
  tail -60 "$ROOT/err.txt" >&2
  exit 1
fi
python3 - "$ROOT/out.json" <<'EOF'
import json, sys
d = json.load(open(sys.argv[1]))
mode = d["summary"]["isolation"]["mode"]
assert mode == "sandbox", "summary.isolation.mode is %r, not 'sandbox'" % mode
assert d["stats"]["functions"] > 0, "nothing was read"
print("ok: isolation.mode = sandbox, %d functions" % d["stats"]["functions"])
EOF
if [ -e "$OUTSIDE/marker" ]; then
  echo "FAIL: the build script wrote outside the sandbox" >&2
  exit 1
fi
echo "ok: nothing was written outside"
