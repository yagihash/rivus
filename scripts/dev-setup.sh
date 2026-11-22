#!/usr/bin/env bash
# Usage: dev-setup.sh   (run it with `make setup`)
# Sets up a new machine so that `make check` (and the Dart tests of `make test`) can run: the Go toolchain of go.mod,
# the Dart SDK that .github/workflows/ci.yml uses, and the `dart pub get` of the packages that the tests read.
# Run it again whenever you like: what is already there is left alone.
#
# Not covered: Rust. The nightly compiler and the isolation of the Rust extractor are not installed (a model for a Go or Dart
# package does not need them). `make test` needs them: see .github/workflows/ci.yml (job `rust`) and docs/rust-isolation.md.
#
# - the versions are pinned below; each download is checked against the sha256 written here, and nothing that is
#   downloaded is run before it is checked (no `curl | sh`)
# - nothing needs sudo: the SDKs go under $RIVUS_DEV_TOOLS (default ~/.local/rivus-dev), so no root is asked for
# - no secret is read or written. Hosts it talks to: go.dev and dl.google.com (only when Go is installed here),
#   proxy.golang.org and sum.golang.org (Go modules and the toolchain that go.mod names), storage.googleapis.com (Dart SDK),
#   pub.dev (Dart packages)
set -euo pipefail

GO_VERSION=1.27.4
DART_VERSION=3.13.6
# sha256 of go${GO_VERSION}.linux-<arch>.tar.gz (go.dev/dl) and dartsdk-linux-<arch>-release.zip (the .sha256sum next to it in dart-archive)
GO_SHA256_amd64=7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840
GO_SHA256_arm64=3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92
DART_SHA256_x64=f80780c01ade17b789cf4dc2425d662f63bdd40eceda53b36c0d46af650de41b
DART_SHA256_arm64=73753a2e009505a2ba0d0c0bd449e248001a01984c805a3fcc19671dbb9616b1

root=$(cd "$(dirname "$0")/.." && pwd)
tools=${RIVUS_DEV_TOOLS:-$HOME/.local/rivus-dev}
cd "$root"

[ "$(uname -s)" = Linux ] || { echo "dev-setup: Linux only (on macOS: brew install go dart-sdk)"; exit 1; }
case "$(uname -m)" in
  x86_64) go_arch=amd64 dart_arch=x64 ;;
  aarch64 | arm64) go_arch=arm64 dart_arch=arm64 ;;
  *) echo "dev-setup: no pinned download for $(uname -m)"; exit 1 ;;
esac
for c in curl sha256sum tar unzip; do command -v "$c" >/dev/null || { echo "dev-setup: $c is needed"; exit 1; }; done

# the pin and go.mod have to agree: a bump of go.mod without the pin here fails, and is not skipped
want_go=$(awk '$1 == "go" { print $2; exit }' go.mod)
case "$GO_VERSION" in
  "$want_go" | "$want_go".*) ;;
  *) echo "dev-setup: go.mod says go $want_go but GO_VERSION is $GO_VERSION: update the pin and its checksums"; exit 1 ;;
esac
ci_dart=$(awk '/dart-lang\/setup-dart/ { f = 1 } f && $1 == "sdk:" { print $2; exit }' .github/workflows/ci.yml)
[ "$ci_dart" = "$DART_VERSION" ] || { echo "dev-setup: ci.yml uses Dart $ci_dart but DART_VERSION is $DART_VERSION: update the pin and its checksums"; exit 1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# download <url> <sha256> <file>: the file is not used until its checksum matches
download() {
  curl --fail --silent --show-error --location --retry 3 --output "$3" "$1"
  echo "$2  $3" | sha256sum --check --status || { echo "dev-setup: the checksum of $1 does not match"; exit 1; }
}

# --- Go ---
# What is there counts when it is the minor version of go.mod (1.27) and not older than GO_VERSION (a newer patch is kept).
# With GOTOOLCHAIN=auto (the default) an older go fetches the toolchain that go.mod names (1.27.0, checked by the Go checksum
# database), which is older than the pin, so the pinned archive is installed then.
go_ok() {
  local v
  v=$(go version 2>/dev/null | awk '{ sub(/^go/, "", $3); print $3 }')
  [ -n "$v" ] || return 1
  case "$v" in "$want_go".*) ;; *) return 1 ;; esac
  [ "${v##*.}" -ge "${GO_VERSION##*.}" ] 2>/dev/null
}
if ! go_ok; then
  echo "dev-setup: installing Go ${GO_VERSION} in ${tools}"
  mkdir -p "$tools"
  sum=GO_SHA256_$go_arch
  download "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" "${!sum}" "$tmp/go.tgz"
  rm -rf "$tools/go-${GO_VERSION}"
  mkdir "$tmp/go"
  tar -xzf "$tmp/go.tgz" -C "$tmp/go"
  mv "$tmp/go/go" "$tools/go-${GO_VERSION}"
  export PATH="$tools/go-${GO_VERSION}/bin:$PATH"
  need_path="$tools/go-${GO_VERSION}/bin"
fi
echo "dev-setup: $(go version)"

# --- Dart ---
dart_bin=$(command -v dart || true)
if [ -z "$dart_bin" ] || ! "$dart_bin" --version 2>&1 | grep -q "Dart SDK version: ${DART_VERSION} "; then
  dart_bin=$tools/dart-sdk-${DART_VERSION}/bin/dart
  if [ ! -x "$dart_bin" ]; then
    echo "dev-setup: installing Dart ${DART_VERSION} in ${tools}"
    mkdir -p "$tools"
    sum=DART_SHA256_$dart_arch
    download "https://storage.googleapis.com/dart-archive/channels/stable/release/${DART_VERSION}/sdk/dartsdk-linux-${dart_arch}-release.zip" "${!sum}" "$tmp/dart.zip"
    mkdir "$tmp/dart"
    unzip -q "$tmp/dart.zip" -d "$tmp/dart"
    rm -rf "$tools/dart-sdk-${DART_VERSION}"
    mv "$tmp/dart/dart-sdk" "$tools/dart-sdk-${DART_VERSION}"
  fi
  export PATH="$(dirname "$dart_bin"):$PATH"
  need_path="${need_path:+$need_path:}$(dirname "$dart_bin")"
fi
echo "dev-setup: $(dart --version 2>&1)"

# --- dart pub get: the packages that the tests and the Dart extractor read (the list of the job `dart` in ci.yml) ---
# every one has a pubspec.lock: the locked versions are used, or the step fails
for d in extractor/dart extractor/dart/test/sample extractor/dart/test/monorepo/app extractor/dart/test/monorepo/shared \
  extractor/dart/test/versions/a extractor/dart/test/versions/b bench/dart-curated; do
  if [ -f "$d/pubspec.yaml" ]; then
    echo "dev-setup: dart pub get in $d"
    (cd "$d" && dart pub get --enforce-lockfile)
  fi
done
# the two of the Dart zoo have no pubspec.lock in the repository (not committed): the versions are resolved now
for d in bench/zoo-dart/cells bench/zoo-dart/exec; do
  echo "dev-setup: dart pub get in $d"
  (cd "$d" && dart pub get)
done

echo
echo "dev-setup: done."
if [ -n "${need_path:-}" ]; then
  echo "Put the tools on PATH (this shell, or your profile):"
  echo "  export PATH=\"${need_path}:\$PATH\""
fi
echo "Then check it works:"
echo "  make check            # about a minute: build, gofmt, go vet, the short tests"
echo "  RIVUS_DART=dart go test ./cmd/rivus   # the tests with the Dart extractor (make test also needs Rust)"
