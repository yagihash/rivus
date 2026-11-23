# The commands to build, check and test rivus. `make` or `make help` lists them.
#
#   make setup   on a new machine: Go, the Dart SDK and `dart pub get` (once; not Rust)
#   make check   before a push (about two minutes): build, gofmt, go vet and the short tests (TestDogfood included: rivus analyses its own code, and new code can break its capture_unbound ceiling)
#   make test    every test, with the Dart and Rust extractors (what CI runs on main); a few minutes
#
# CI runs the same tests (a pull request with -short, main and the weekly run without): .github/workflows/ci.yml.

.DEFAULT_GOAL := help
.PHONY: help setup build check test zoo-loosened zoo-dart-gen model-diff model docs-gen

# the extractors that `make test` needs (a language whose extractor is not there FAILS, it is not skipped)
RIVUS_DART ?= dart
RIVUS_RUST ?= $(CURDIR)/extractor/rust/target/debug/rivus-rust

help: ## list the commands
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | sed -E 's/^([a-z-]+):.*## (.*)/  make \1\t\2/' | expand -t 24

setup: ## install Go and Dart if they are missing, and run dart pub get (Linux; not Rust). Run `make check` after it
	bash scripts/dev-setup.sh

build: ## build every package
	go build ./...

check: ## before a push (about two minutes): build, gofmt, go vet, the short tests (TestDogfood included)
	go build ./...
	@bad="$$(gofmt -l cmd internal bench)"; if [ -n "$$bad" ]; then echo "gofmt: not formatted:"; echo "$$bad"; exit 1; fi
	go vet ./...
	go test -short -count=1 ./...

test: ## every test, with the Dart and Rust extractors (what CI runs on main)
	@command -v "$(firstword $(RIVUS_DART))" >/dev/null || { echo "make test: no dart ($(RIVUS_DART))"; exit 1; }
	@[ -x "$(RIVUS_RUST)" ] || { echo "make test: no Rust extractor at $(RIVUS_RUST) (cd extractor/rust && cargo build)"; exit 1; }
	go vet ./...
	RIVUS_DART="$(RIVUS_DART)" RIVUS_RUST="$(RIVUS_RUST)" RIVUS_REQUIRE_DART=1 RIVUS_REQUIRE_RUST=1 go test -count=1 -timeout 40m ./...

zoo-loosened: ## check that what the commits change in the zoo marks (bench/zoo/marks.d, against the merge base) is not a loosening without a "Zoo-Loosened:" line (BASE=<commit> HEAD=<commit>)
	@[ -n "$(BASE)" ] && [ -n "$(HEAD)" ] || { echo "usage: make zoo-loosened BASE=<commit> HEAD=<commit>"; exit 1; }
	bash scripts/zoo-loosened-check.sh "$(BASE)" "$(HEAD)"

zoo-dart-gen: ## write bench/zoo-dart/exec/lib/cases.dart and the two pubspec.yaml of the Dart zoo (generated, not committed) from the directories of the cells and of the skeleton packages
	go test ./cmd/rivus -run '^TestZooDartGenerate$$' -count=1 -update

model-diff: ## the differential test of the model of a package: PKG=<Go import path | package:name> [MODEL=<file>] (bench/model-diff, docs/MODEL-RUNBOOK.md); fetches the package, does not run it
	@[ -n "$(PKG)" ] || { echo "usage: make model-diff PKG=strings   (or PKG=package:http, MODEL=internal/model/builtin/dart-http.yaml)"; exit 1; }
	go run ./bench/model-diff -pkg "$(PKG)" $(if $(MODEL),-model "$(MODEL)")

model: ## the steps of making the model of a Go package: PKG=<import path> [VERSION=v1.2.3] [LANG=go] [ARGS="-diff -write-verified"] (bench/model-author, docs/MODEL-RUNBOOK.md); exit 70 = a candidate has no row and is not in rejected:; fetches the package, does not run it
	@[ -n "$(PKG)" ] || { echo "usage: make model PKG=github.com/jmoiron/sqlx VERSION=v1.4.0 LANG=go   (ARGS=\"-recall\" for the recall of layer 1; ARGS=\"-gen-skeleton\" makes the skeleton and the cell template for the Go zoo)"; exit 1; }
	go run ./bench/model-author -pkg "$(PKG)" $(if $(VERSION),-version "$(VERSION)") -lang $(if $(filter command line,$(origin LANG)),$(LANG),go) $(ARGS)

docs-gen: ## print the tables of the issues and the ADRs (from docs/issues/*.md, docs/adr/*.md); WHAT=issues or WHAT=adr for one; OUT=docs/TABLE.md writes it (ignored by git)
	go run ./scripts/docs-gen $(if $(OUT),-o $(OUT)) $(WHAT)
