# The commands to build, check and test rivus. `make` or `make help` lists them.
#
#   make check   before a push (about a minute): build, gofmt, go vet and the short tests
#   make test    every test, with the Dart and Rust extractors (what CI runs on main); a few minutes
#
# CI runs the same tests (a pull request with -short, main and the weekly run without): .github/workflows/ci.yml.

.DEFAULT_GOAL := help
.PHONY: help build check test zoo-loosened

# the extractors that `make test` needs (a language whose extractor is not there FAILS, it is not skipped)
RIVUS_DART ?= dart
RIVUS_RUST ?= $(CURDIR)/extractor/rust/target/debug/rivus-rust

help: ## list the commands
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | sed -E 's/^([a-z-]+):.*## (.*)/  make \1\t\2/' | expand -t 24

build: ## build every package
	go build ./...

check: ## before a push (about a minute): build, gofmt, go vet, the short tests
	go build ./...
	@bad="$$(gofmt -l cmd internal bench)"; if [ -n "$$bad" ]; then echo "gofmt: not formatted:"; echo "$$bad"; exit 1; fi
	go vet ./...
	go test -short -count=1 -skip 'TestDogfood' ./...

test: ## every test, with the Dart and Rust extractors (what CI runs on main)
	@command -v "$(firstword $(RIVUS_DART))" >/dev/null || { echo "make test: no dart ($(RIVUS_DART))"; exit 1; }
	@[ -x "$(RIVUS_RUST)" ] || { echo "make test: no Rust extractor at $(RIVUS_RUST) (cd extractor/rust && cargo build)"; exit 1; }
	go vet ./...
	RIVUS_DART="$(RIVUS_DART)" RIVUS_RUST="$(RIVUS_RUST)" RIVUS_REQUIRE_DART=1 RIVUS_REQUIRE_RUST=1 go test -count=1 -timeout 40m ./...

zoo-loosened: ## check that a commit that loosens the zoo has a "Zoo-Loosened:" line (BASE=<commit> HEAD=<commit>)
	@[ -n "$(BASE)" ] && [ -n "$(HEAD)" ] || { echo "usage: make zoo-loosened BASE=<commit> HEAD=<commit>"; exit 1; }
	bash scripts/zoo-loosened-check.sh "$(BASE)" "$(HEAD)"
