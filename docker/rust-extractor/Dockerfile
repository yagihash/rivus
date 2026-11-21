# The image the Rust extractor of rivus runs in when it reads code that is not trusted (ADR-064,
# docs/rust-isolation.md): the pinned nightly, a C linker, and the extractor. Nothing of the project that is read is
# in it. Build it from the trusted source (this repository), with the context extractor/rust:
#
#   docker build -f docker/rust-extractor/Dockerfile -t rivus-rust-extractor:nightly-2026-11-15 extractor/rust
#
# The base is pinned by digest (debian:bookworm-slim). The nightly is pinned by date, the same as
# extractor/rust/rust-toolchain.toml, which the build checks.
FROM debian@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc AS toolchain
ARG TOOLCHAIN=nightly-2026-11-15
ENV RUSTUP_HOME=/opt/rustup CARGO_HOME=/opt/cargo PATH=/opt/cargo/bin:$PATH
# What the build scripts of the dependencies that are in common use ask for: a C and C++ compiler, make (jemalloc),
# cmake, pkg-config, perl (a vendored OpenSSL). Not more: a project that needs another tool does not build here.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl build-essential pkg-config cmake perl \
 && rm -rf /var/lib/apt/lists/*
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path --profile minimal --default-toolchain none \
 && rustup toolchain install "$TOOLCHAIN" --profile minimal -c rustc-dev -c llvm-tools -c rust-src \
 && rustup default "$TOOLCHAIN" \
 && chmod -R a-w /opt/rustup /opt/cargo

FROM toolchain AS build
ARG TOOLCHAIN=nightly-2026-11-15
COPY . /extractor
WORKDIR /extractor
# the toolchain of the image and the one the extractor is pinned to are one
RUN grep -q "channel = \"$TOOLCHAIN\"" rust-toolchain.toml \
 && CARGO_HOME=/tmp/cargo-build CARGO_TARGET_DIR=/build cargo build --release --locked

FROM toolchain
ARG TOOLCHAIN=nightly-2026-11-15
# a marker that the code that is hostile on purpose (extractor/rust/test/hostile) looks for: it does nothing without it
RUN echo "rivus rust extractor isolation image" > /etc/rivus-isolated \
 && mkdir -p /cargo-home /work && chown 10001:10001 /cargo-home /work
COPY --from=build /build/release/rivus-rust /usr/local/bin/rivus-rust
ENV RUSTUP_HOME=/opt/rustup RUSTUP_TOOLCHAIN=nightly-2026-11-15 CARGO_HOME=/cargo-home \
    PATH=/usr/local/bin:/opt/cargo/bin:/usr/bin:/bin HOME=/work TMPDIR=/work
USER 10001:10001
LABEL org.rivus.image="rust-extractor" org.rivus.toolchain="nightly-2026-11-15"
ENTRYPOINT ["rivus-rust"]
